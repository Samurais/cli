package commands

import (
	"cf/api"
	"cf/configuration"
	"cf/models"
	"cf/net"
	"cf/requirements"
	"cf/terminal"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"strconv"
	"strings"
)

const maxLoginTries = 3
const maxChoices = 50

type Login struct {
	ui            terminal.UI
	config        configuration.ReadWriter
	authenticator api.AuthenticationRepository
	endpointRepo  api.EndpointRepository
	orgRepo       api.OrganizationRepository
	spaceRepo     api.SpaceRepository
}

func NewLogin(ui terminal.UI,
	config configuration.ReadWriter,
	authenticator api.AuthenticationRepository,
	endpointRepo api.EndpointRepository,
	orgRepo api.OrganizationRepository,
	spaceRepo api.SpaceRepository) (cmd Login) {

	cmd.ui = ui
	cmd.config = config
	cmd.authenticator = authenticator
	cmd.endpointRepo = endpointRepo
	cmd.orgRepo = orgRepo
	cmd.spaceRepo = spaceRepo

	return
}

const userSkippedInput string = "user_skipped_input"

func (cmd Login) GetRequirements(reqFactory requirements.Factory, c *cli.Context) (reqs []requirements.Requirement, err error) {
	return
}

func (cmd Login) Run(c *cli.Context) {
	oldUserName := cmd.config.Username()

	apiResponse := cmd.setApi(c)
	if apiResponse.IsNotSuccessful() {
		cmd.ui.Failed("Invalid API endpoint.\n%s", apiResponse.Message)
		return
	}

	apiResponse = cmd.authenticate(c)
	if apiResponse.IsNotSuccessful() {
		cmd.ui.Failed("Unable to authenticate.")
		return
	}

	userChanged := (cmd.config.Username() != oldUserName && oldUserName != "")

	err := cmd.setOrganization(c, userChanged)
	shouldSkipSpace := err != nil && err.Error() == userSkippedInput

	if err != nil && !shouldSkipSpace {
		cmd.ui.Failed(err.Error())
		return
	}

	if !shouldSkipSpace {
		err = cmd.setSpace(c, userChanged)
		if err != nil && err.Error() != userSkippedInput {
			cmd.ui.Failed(err.Error())
			return
		}
	}

	cmd.ui.ShowConfiguration(cmd.config)
	return
}

func (cmd Login) setApi(c *cli.Context) (apiResponse net.ApiResponse) {
	api := c.String("a")
	if api == "" {
		api = cmd.config.ApiEndpoint()
	}

	if api == "" {
		api = cmd.ui.Ask("API endpoint%s", terminal.PromptColor(">"))
	} else {
		cmd.ui.Say("API endpoint: %s", terminal.EntityNameColor(api))
	}

	endpoint, apiResponse := cmd.endpointRepo.UpdateEndpoint(api)

	if !strings.HasPrefix(endpoint, "https://") {
		cmd.ui.Say(terminal.WarningColor("Warning: Insecure http API endpoint detected: secure https API endpoints are recommended\n"))
	}

	return
}

func (cmd Login) authenticate(c *cli.Context) (apiResponse net.ApiResponse) {
	username := c.String("u")
	if username == "" {
		username = cmd.ui.Ask("Username%s", terminal.PromptColor(">"))
	}

	password := c.String("p")

	for i := 0; i < maxLoginTries; i++ {
		if password == "" || i > 0 {
			password = cmd.ui.AskForPassword("Password%s", terminal.PromptColor(">"))
		}

		cmd.ui.Say("Authenticating...")

		apiResponse = cmd.authenticator.Authenticate(map[string]string{
			"username": username,
			"password": password,
		})

		if apiResponse.IsSuccessful() {
			cmd.ui.Ok()
			cmd.ui.Say("")
			break
		}

		cmd.ui.Say(apiResponse.Message)
	}
	return
}

func (cmd Login) setOrganization(c *cli.Context, userChanged bool) (err error) {
	orgName := c.String("o")

	if orgName == "" {
		// If the user is changing, clear out the org
		if userChanged {
			cmd.config.SetOrganizationFields(models.OrganizationFields{})
		}

		// Reuse org in config
		if cmd.config.HasOrganization() && !userChanged {
			return
		}

		availableOrgs := []models.Organization{}
		apiResponse := cmd.orgRepo.ListOrgs(func(o models.Organization) bool {
			availableOrgs = append(availableOrgs, o)
			return len(availableOrgs) < maxChoices
		})

		if apiResponse.IsNotSuccessful() {
			err = errors.New(fmt.Sprintf("Error finding avilable orgs\n%s", apiResponse.Message))
			return
		}

		if len(availableOrgs) == 1 {
			return cmd.targetOrganization(availableOrgs[0])
		}

		orgName = cmd.promptForOrgName(availableOrgs)
	}

	if orgName == "" {
		cmd.ui.Say("")
		err = errors.New(userSkippedInput)
		return
	}

	var org models.Organization
	var apiResponse net.ApiResponse
	org, apiResponse = cmd.orgRepo.FindByName(orgName)
	if apiResponse.IsNotSuccessful() {
		err = errors.New(apiResponse.Message)
		cmd.ui.Failed("Error finding org %s\n%s", terminal.EntityNameColor(orgName), err)
		return
	}

	return cmd.targetOrganization(org)
}

func (cmd Login) promptForOrgName(orgs []models.Organization) string {
	orgNames := []string{}
	for _, org := range orgs {
		orgNames = append(orgNames, org.Name)
	}

	return cmd.promptForName(orgNames, "Select an org (or press enter to skip):", "Org")
}

func (cmd Login) targetOrganization(org models.Organization) (err error) {
	cmd.config.SetOrganizationFields(org.OrganizationFields)
	cmd.ui.Say("Targeted org %s\n", terminal.EntityNameColor(org.Name))
	return
}

func (cmd Login) setSpace(c *cli.Context, userChanged bool) (err error) {
	spaceName := c.String("s")

	if spaceName == "" {
		// If user is changing, clear the space
		if userChanged {
			cmd.config.SetSpaceFields(models.SpaceFields{})
		}
		// Reuse space in config
		if cmd.config.HasSpace() && !userChanged {
			return
		}

		var availableSpaces []models.Space
		apiResponse := cmd.spaceRepo.ListSpaces(func(space models.Space) bool {
			availableSpaces = append(availableSpaces, space)
			return (len(availableSpaces) < maxChoices)
		})

		if apiResponse.IsNotSuccessful() {
			err = errors.New(fmt.Sprintf("Error finding available spaces\n%s", apiResponse.Message))
			cmd.ui.Failed(err.Error())
			return
		}

		// Target only space if possible
		if len(availableSpaces) == 1 {
			return cmd.targetSpace(availableSpaces[0])
		}

		spaceName = cmd.promptForSpaceName(availableSpaces)
	}

	if spaceName == "" {
		cmd.ui.Say("")
		err = errors.New(userSkippedInput)
		return
	}

	var space models.Space
	var apiResponse net.ApiResponse
	space, apiResponse = cmd.spaceRepo.FindByName(spaceName)
	if apiResponse.IsNotSuccessful() {
		err = errors.New(fmt.Sprintf("Error finding space %s\n%s", terminal.EntityNameColor(spaceName), apiResponse.Message))
		cmd.ui.Failed(err.Error())
		return
	}

	err = cmd.targetSpace(space)
	return
}

func (cmd Login) promptForSpaceName(spaces []models.Space) string {
	spaceNames := []string{}
	for _, space := range spaces {
		spaceNames = append(spaceNames, space.Name)
	}

	return cmd.promptForName(spaceNames, "Select a space (or press enter to skip):", "Space")
}

func (cmd Login) targetSpace(space models.Space) (err error) {
	cmd.config.SetSpaceFields(space.SpaceFields)
	cmd.ui.Say("Targeted space %s\n", terminal.EntityNameColor(space.Name))
	return
}

func (cmd Login) promptForName(names []string, listPrompt, itemPrompt string) string {
	nameIndex := 0
	var nameString string
	for nameIndex < 1 || nameIndex > len(names) {
		var err error

		// list header
		cmd.ui.Say(listPrompt)

		// only display list if it is shorter than maxChoices
		if len(names) < maxChoices {
			for i, name := range names {
				cmd.ui.Say("%d. %s", i+1, name)
			}
		} else {
			cmd.ui.Say("There are too many options to display, please type in the name.")
		}

		nameString = cmd.ui.Ask("%s%s", itemPrompt, terminal.PromptColor(">"))
		if nameString == "" {
			return ""
		}

		nameIndex, err = strconv.Atoi(nameString)

		if err != nil {
			nameIndex = 1
			return nameString
		}
	}

	return names[nameIndex-1]
}
