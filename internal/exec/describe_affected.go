package exec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	giturl "github.com/kubescape/go-git-url"
	"github.com/spf13/cobra"

	cfg "github.com/cloudposse/atmos/pkg/config"
	"github.com/cloudposse/atmos/pkg/schema"
	u "github.com/cloudposse/atmos/pkg/utils"
)

// ExecuteDescribeAffectedCmd executes `describe affected` command
func ExecuteDescribeAffectedCmd(cmd *cobra.Command, args []string) error {
	info, err := processCommandLineArgs("", cmd, args, nil)
	if err != nil {
		return err
	}

	cliConfig, err := cfg.InitCliConfig(info, true)
	if err != nil {
		return err
	}

	err = ValidateStacks(cliConfig)
	if err != nil {
		return err
	}

	// Process flags
	flags := cmd.Flags()

	ref, err := flags.GetString("ref")
	if err != nil {
		return err
	}

	sha, err := flags.GetString("sha")
	if err != nil {
		return err
	}

	repoPath, err := flags.GetString("repo-path")
	if err != nil {
		return err
	}

	format, err := flags.GetString("format")
	if err != nil {
		return err
	}

	if format != "" && format != "yaml" && format != "json" {
		return fmt.Errorf("invalid '--format' flag '%s'. Valid values are 'json' (default) and 'yaml'", format)
	}

	if format == "" {
		format = "json"
	}

	file, err := flags.GetString("file")
	if err != nil {
		return err
	}

	verbose, err := flags.GetBool("verbose")
	if err != nil {
		return err
	}

	sshKeyPath, err := flags.GetString("ssh-key")
	if err != nil {
		return err
	}

	sshKeyPassword, err := flags.GetString("ssh-key-password")
	if err != nil {
		return err
	}

	includeSpaceliftAdminStacks, err := flags.GetBool("include-spacelift-admin-stacks")
	if err != nil {
		return err
	}

	includeDependents, err := flags.GetBool("include-dependents")
	if err != nil {
		return err
	}

	includeSettings, err := flags.GetBool("include-settings")
	if err != nil {
		return err
	}

	upload, err := flags.GetBool("upload")
	if err != nil {
		return err
	}

	cloneTargetRef, err := flags.GetBool("clone-target-ref")
	if err != nil {
		return err
	}

	if repoPath != "" && (ref != "" || sha != "" || sshKeyPath != "" || sshKeyPassword != "") {
		return errors.New("if the '--repo-path' flag is specified, the '--ref', '--sha', '--ssh-key' and '--ssh-key-password' flags can't be used")
	}

	// When uploading, always include dependents and settings for all affected components
	if upload {
		includeDependents = true
		includeSettings = true
	}

	if verbose {
		cliConfig.Logs.Level = u.LogLevelTrace
	}

	var affected []schema.Affected
	var headHead, baseHead *plumbing.Reference
	var repoUrl string

	if repoPath != "" {
		affected, headHead, baseHead, repoUrl, err = ExecuteDescribeAffectedWithTargetRepoPath(cliConfig, repoPath, verbose, includeSpaceliftAdminStacks, includeSettings)
	} else if cloneTargetRef {
		affected, headHead, baseHead, repoUrl, err = ExecuteDescribeAffectedWithTargetRefClone(cliConfig, ref, sha, sshKeyPath, sshKeyPassword, verbose, includeSpaceliftAdminStacks, includeSettings)
	} else {
		affected, headHead, baseHead, repoUrl, err = ExecuteDescribeAffectedWithTargetRefCheckout(cliConfig, ref, sha, verbose, includeSpaceliftAdminStacks, includeSettings)
	}

	if err != nil {
		return err
	}

	// Add dependent components and stacks for each affected component
	if len(affected) > 0 && includeDependents {
		err = addDependentsToAffected(cliConfig, &affected, includeSettings)
		if err != nil {
			return err
		}
	}

	u.LogTrace(cliConfig, fmt.Sprintf("\nAffected components and stacks: \n"))

	err = printOrWriteToFile(format, file, affected)
	if err != nil {
		return err
	}

	// Upload the affected components and stacks to a specified endpoint
	// https://www.digitalocean.com/community/tutorials/how-to-make-http-requests-in-go
	if upload {
		baseUrl := os.Getenv(cfg.AtmosProBaseUrlEnvVarName)
		if baseUrl == "" {
			baseUrl = cfg.AtmosProDefaultBaseUrl
		}
		endpoint := os.Getenv(cfg.AtmosProEndpointEnvVarName)
		if endpoint == "" {
			endpoint = cfg.AtmosProDefaultEndpoint
		}
		url := fmt.Sprintf("%s/%s", baseUrl, endpoint)

		// Parse the repo URL
		gitURL, err := giturl.NewGitURL(repoUrl)
		if err != nil {
			return err
		}

		body := map[string]any{
			"head_sha":   headHead.Hash().String(),
			"base_sha":   baseHead.Hash().String(),
			"repo_url":   repoUrl,
			"repo_name":  gitURL.GetRepoName(),
			"repo_owner": gitURL.GetOwnerName(),
			"repo_host":  gitURL.GetHostName(),
			"stacks":     affected,
			"config":     cliConfig.Integrations.Pro,
		}

		bodyJson, err := u.ConvertToJSON(body)
		if err != nil {
			return err
		}

		u.LogTrace(cliConfig, fmt.Sprintf("\nUploading the affected components and stacks to %s", url))

		bodyReader := bytes.NewReader([]byte(bodyJson))
		req, err := http.NewRequest(http.MethodPost, url, bodyReader)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")

		// Authorization header
		// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Authorization
		token := os.Getenv(cfg.AtmosProTokenEnvVarName)
		if token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}

		client := http.Client{
			Timeout: 10 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				u.LogError(err)
			}
		}(resp.Body)

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
			err = fmt.Errorf("\nError uploading the affected components and stacks to %s\nStatus: %s\n", url, resp.Status)
			return err
		}

		u.LogTrace(cliConfig, fmt.Sprintf("\nUploaded the affected components and stacks to %s\n", url))
	}

	return nil
}
