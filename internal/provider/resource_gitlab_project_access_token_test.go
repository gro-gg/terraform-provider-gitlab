package provider

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/onsi/gomega"
	gitlab "github.com/xanzy/go-gitlab"
)

func TestAccGitlabProjectAccessToken_basic(t *testing.T) {
	var pat testAccGitlabProjectAccessTokenWrapper
	rInt := acctest.RandInt()

	testAccGitlabProjectStart(t)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckGitlabProjectAccessTokenDestroy,
		Steps: []resource.TestStep{
			// Create a project and a Project Access Token
			{
				Config: testAccGitlabProjectAccessTokenConfig(rInt),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabProjectAccessTokenExists("gitlab_project_access_token.bar", &pat),
					testAccCheckGitlabProjectAccessTokenAttributes(&pat, &testAccGitlabProjectAccessTokenExpectedAttributes{
						name:        "my project token",
						scopes:      map[string]bool{"read_repository": true, "api": true, "write_repository": true, "read_api": true},
						expiresAt:   "2022-04-01",
						accessLevel: accessLevelValueToName[gitlab.MaintainerPermissions], // default permission on gitlab side when unspecified
					}),
				),
			},
			// Update the Project Access Token to change the parameters
			{
				Config: testAccGitlabProjectAccessTokenUpdateConfig(rInt),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabProjectAccessTokenExists("gitlab_project_access_token.bar", &pat),
					testAccCheckGitlabProjectAccessTokenAttributes(&pat, &testAccGitlabProjectAccessTokenExpectedAttributes{
						name:      "my new project token",
						scopes:    map[string]bool{"read_repository": false, "api": true, "write_repository": false, "read_api": false},
						expiresAt: "2022-05-01",
					}),
				),
			},
			// Add a CICD variable with Project Access Token value
			{
				Config: testAccGitlabProjectAccessTokenUpdateConfigWithCICDvar(rInt),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabProjectAccessTokenExists("gitlab_project_access_token.bar", &pat),
					testAccCheckGitlabProjectVariableExists("gitlab_project_variable.var"),
					testAccCheckGitlabProjectAccessTokenAttributes(&pat, &testAccGitlabProjectAccessTokenExpectedAttributes{
						name:      "my new project token",
						scopes:    map[string]bool{"read_repository": false, "api": true, "write_repository": false, "read_api": false},
						expiresAt: "2022-05-01",
					}),
				),
			},
			//Restore Project Access Token initial parameters
			{
				Config: testAccGitlabProjectAccessTokenConfig(rInt),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabProjectAccessTokenExists("gitlab_project_access_token.bar", &pat),
					testAccCheckGitlabProjectAccessTokenAttributes(&pat, &testAccGitlabProjectAccessTokenExpectedAttributes{
						name:      "my project token",
						scopes:    map[string]bool{"read_repository": true, "api": true, "write_repository": true, "read_api": true},
						expiresAt: "2022-04-01",
					}),
				),
			},
			// Verify import
			{
				ResourceName:      "gitlab_project_access_token.bar",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// the token is only known during creating. We explicitly mention this limitation in the docs.
					"token",
				},
			},
			//Destroy Project Access Token
			{
				Config: testAccGitlabProjectAccessTokenDestroyToken(rInt),
				Check:  testAccCheckGitlabProjectAccessTokenDoesNotExist(&pat),
			},
		},
	})
}

func TestAccGitlabProjectAccessToken_accessLevel(t *testing.T) {
	var pat testAccGitlabProjectAccessTokenWrapper
	rInt := acctest.RandInt()

	testAccGitlabProjectStart(t)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckGitlabProjectAccessTokenDestroy,
		Steps: []resource.TestStep{
			// Create a project and a Project Access Token
			{
				Config: testAccGitlabProjectAccessTokenConfigWithAccessLevel(rInt, accessLevelValueToName[gitlab.MaintainerPermissions]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabProjectAccessTokenExists("gitlab_project_access_token.bar", &pat),
					testAccCheckGitlabProjectAccessTokenAttributes(&pat, &testAccGitlabProjectAccessTokenExpectedAttributes{
						name:        "my project token",
						scopes:      map[string]bool{"read_repository": true, "api": true, "write_repository": true, "read_api": true},
						expiresAt:   "2022-04-01",
						accessLevel: accessLevelValueToName[gitlab.MaintainerPermissions],
					}),
				),
			},
			// Update the Project Access Token to change the parameters
			{
				Config: testAccGitlabProjectAccessTokenConfigWithAccessLevel(rInt, accessLevelValueToName[gitlab.DeveloperPermissions]),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckGitlabProjectAccessTokenExists("gitlab_project_access_token.bar", &pat),
					testAccCheckGitlabProjectAccessTokenAttributes(&pat, &testAccGitlabProjectAccessTokenExpectedAttributes{
						name:        "my project token",
						scopes:      map[string]bool{"read_repository": true, "api": true, "write_repository": true, "read_api": true},
						expiresAt:   "2022-04-01",
						accessLevel: accessLevelValueToName[gitlab.DeveloperPermissions],
					}),
				),
			},
			// Verify import
			{
				ResourceName:      "gitlab_project_access_token.bar",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// the token is only known during creating. We explicitly mention this limitation in the docs.
					"token",
				},
			},
		}})
}

func testAccCheckGitlabProjectAccessTokenDoesNotExist(pat *testAccGitlabProjectAccessTokenWrapper) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return gomega.InterceptGomegaFailure(func() {
			gomega.Eventually(func() error {
				tokens, _, err := testGitlabClient.ProjectAccessTokens.ListProjectAccessTokens(pat.project, nil)
				if err != nil {
					return err
				}

				for _, token := range tokens {
					if token.ID == pat.pat.ID {
						return fmt.Errorf("Found token %d for project %s (tokens found: %d)", token.ID, pat.project, len(tokens))
					}
				}

				return nil
			}).WithTimeout(time.Second * 10).WithPolling(time.Second * 2).Should(gomega.Succeed())
		})
	}
}

func testAccCheckGitlabProjectAccessTokenExists(n string, pat *testAccGitlabProjectAccessTokenWrapper) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not Found: %s", n)
		}

		project, PATstring, err := parseTwoPartID(rs.Primary.ID)
		if err != nil {
			return fmt.Errorf("Error parsing ID: %s", rs.Primary.ID)
		}
		projectAccessTokenID, err := strconv.Atoi(PATstring)
		if err != nil {
			return fmt.Errorf("%s cannot be converted to int", PATstring)
		}

		repoName := rs.Primary.Attributes["project"]
		if repoName == "" {
			return fmt.Errorf("No project ID is set")
		}
		if repoName != project {
			return fmt.Errorf("Project [%s] in project identifier [%s] it's different from project stored into the state [%s]", project, rs.Primary.ID, repoName)
		}

		tokens, _, err := testGitlabClient.ProjectAccessTokens.ListProjectAccessTokens(repoName, nil)
		if err != nil {
			return err
		}

		for _, token := range tokens {
			if token.ID == projectAccessTokenID {
				pat.pat = token
				pat.project = repoName
				pat.token = rs.Primary.Attributes["token"]
				return nil
			}
		}
		return fmt.Errorf("Project Access Token does not exist")
	}
}

type testAccGitlabProjectAccessTokenExpectedAttributes struct {
	name        string
	scopes      map[string]bool
	expiresAt   string
	accessLevel string
}

type testAccGitlabProjectAccessTokenWrapper struct {
	pat     *gitlab.ProjectAccessToken
	project string
	token   string
}

func testAccCheckGitlabProjectAccessTokenAttributes(patWrap *testAccGitlabProjectAccessTokenWrapper, want *testAccGitlabProjectAccessTokenExpectedAttributes) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		pat := patWrap.pat
		if pat.Name != want.name {
			return fmt.Errorf("got Name %q; want %q", pat.Name, want.name)
		}

		if pat.ExpiresAt.String() != want.expiresAt {
			return fmt.Errorf("got ExpiresAt %q; want %q", pat.ExpiresAt.String(), want.expiresAt)
		}

		for _, scope := range pat.Scopes {
			if !want.scopes[scope] {
				return fmt.Errorf("got a not wanted Scope %q, received %v", scope, pat.Scopes)
			}
			want.scopes[scope] = false
		}
		for k, v := range want.scopes {
			if v {
				return fmt.Errorf("not got a wanted Scope %q, received %v", k, pat.Scopes)
			}
		}

		git, err := gitlab.NewClient(patWrap.token, gitlab.WithBaseURL(testGitlabClient.BaseURL().String()))
		if err != nil {
			return fmt.Errorf("Cannot use the token to instantiate a new client %s", err)
		}
		_, _, err = git.ProjectMembers.ListAllProjectMembers(patWrap.project, nil)
		if err != nil {
			return fmt.Errorf("Cannot use the token to perform an API call %s", err)
		}

		return nil
	}
}

func testAccCheckGitlabProjectAccessTokenDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "gitlab_project" {
			continue
		}

		gotRepo, resp, err := testGitlabClient.Projects.GetProject(rs.Primary.ID, nil)
		if err == nil {
			if gotRepo != nil && fmt.Sprintf("%d", gotRepo.ID) == rs.Primary.ID {
				if gotRepo.MarkedForDeletionAt == nil {
					return fmt.Errorf("Repository still exists")
				}
			}
		}
		if resp.StatusCode != 404 {
			return err
		}
		return nil
	}
	return nil
}

func testAccGitlabProjectAccessTokenConfig(rInt int) string {
	return fmt.Sprintf(`
resource "gitlab_project" "foo" {
  name = "foo-%d"
  description = "Terraform acceptance tests"

  # So that acceptance tests can be run in a gitlab organization
  # with no billing
  visibility_level = "public"
}

resource "gitlab_project_access_token" "bar" {
  name = "my project token"
  project = gitlab_project.foo.id
  expires_at = "2022-04-01"
  scopes = ["read_repository" , "api", "write_repository", "read_api"]
}
	`, rInt)
}

func testAccGitlabProjectAccessTokenDestroyToken(rInt int) string {
	return fmt.Sprintf(`
resource "gitlab_project" "foo" {
  name = "foo-%d"
  description = "Terraform acceptance tests"

  # So that acceptance tests can be run in a gitlab organization
  # with no billing
  visibility_level = "public"
}
	`, rInt)
}

func testAccGitlabProjectAccessTokenUpdateConfig(rInt int) string {
	return fmt.Sprintf(`
resource "gitlab_project" "foo" {
  name = "foo-%d"
  description = "Terraform acceptance tests"

  # So that acceptance tests can be run in a gitlab organization
  # with no billing
  visibility_level = "public"
}

resource "gitlab_project_access_token" "bar" {
  name = "my new project token"
  project = gitlab_project.foo.id
  expires_at = "2022-05-01"
  scopes = ["api"]
}
	`, rInt)
}

func testAccGitlabProjectAccessTokenUpdateConfigWithCICDvar(rInt int) string {
	return fmt.Sprintf(`
resource "gitlab_project" "foo" {
  name = "foo-%d"
  description = "Terraform acceptance tests"

  # So that acceptance tests can be run in a gitlab organization
  # with no billing
  visibility_level = "public"
}

resource "gitlab_project_access_token" "bar" {
  name = "my new project token"
  project = gitlab_project.foo.id
  expires_at = "2022-05-01"
  scopes = ["api"]
}


resource "gitlab_project_variable" "var" {
  project   = gitlab_project.foo.id
  key       = "my_proj_access_token"
  value     = gitlab_project_access_token.bar.token
 }

	`, rInt)
}

func testAccGitlabProjectAccessTokenConfigWithAccessLevel(rInt int, level string) string {
	return fmt.Sprintf(`
resource "gitlab_project" "foo" {
  name = "foo-%d"
  description = "Terraform acceptance tests"

  # So that acceptance tests can be run in a gitlab organization
  # with no billing
  visibility_level = "public"
}

resource "gitlab_project_access_token" "bar" {
  name = "my project token"
  project = gitlab_project.foo.id
  expires_at = "2022-04-01"
  scopes = ["read_repository" , "api", "write_repository", "read_api"]
  access_level = "%s"
}
	`, rInt, level)
}
