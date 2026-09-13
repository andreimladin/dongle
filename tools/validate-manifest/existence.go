// existence.go implements layer (b) of validate-manifest: confirming a
// package exists at a given version in the Azure Artifacts feed a manifest
// names — a metadata-only query, never a full package download.
//
// The azure-devops CLI extension's "universal" command group only exposes
// `download` and `publish` (mirroring internal/plugincmd's downloadArtifact,
// the same one dongle plugin install shells out to) — there is no plain
// existence/show verb, and download pulls the full package content, which
// is too slow to run per-platform on every manifest. Instead this shells
// out to `az rest` against the Azure DevOps Artifacts "Packages" list API,
// which accepts the feed by name and returns package/version metadata
// without transferring package content.
//
// TODO: this targets Azure DevOps Services (feeds.dev.azure.com,
// api-version 7.1). If validate-manifest ever needs to run against an
// on-prem Azure DevOps Server instead, the base URL and api-version below
// are the only things that need to change.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/andreimladin/dongle/internal/index"
)

// azureDevOpsResourceID is the AAD resource ID for Azure DevOps. `az rest`
// defaults to an ARM token, which Azure DevOps' own APIs reject, so it must
// be requested explicitly.
const azureDevOpsResourceID = "499b84ac-1321-427f-aa17-267ca6975798"

type packagesResponse struct {
	Value []struct {
		Name     string `json:"name"`
		Versions []struct {
			Version   string `json:"version"`
			IsDeleted bool   `json:"isDeleted"`
		} `json:"versions"`
	} `json:"value"`
}

// packageExists reports whether packageName@version exists (and is not
// deleted) in feed. feed carries the organization/project/feed coordinates
// straight from the manifest that declared the package — this never
// consults a global/configured feed.
func packageExists(feed index.Feed, packageName, version string) (bool, error) {
	body, err := azRestGet(packagesURL(feed, packageName))
	if err != nil {
		return false, err
	}

	var resp packagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("parsing feed API response: %w", err)
	}

	for _, pkg := range resp.Value {
		if pkg.Name != packageName {
			continue
		}
		for _, v := range pkg.Versions {
			if v.Version == version && !v.IsDeleted {
				return true, nil
			}
		}
	}
	return false, nil
}

// packagesURL builds the Feeds "get packages" query, filtered to the one
// package name we care about, including its versions.
func packagesURL(feed index.Feed, packageName string) string {
	base := fmt.Sprintf("https://feeds.dev.azure.com/%s", url.PathEscape(feed.Organization))
	if feed.Project != "" {
		base += "/" + url.PathEscape(feed.Project)
	}
	q := url.Values{}
	q.Set("packageNameQuery", packageName)
	q.Set("includeAllVersions", "true")
	q.Set("api-version", "7.1")
	return fmt.Sprintf("%s/_apis/packaging/Feeds/%s/packages?%s",
		base, url.PathEscape(feed.Feed), q.Encode())
}

func azRestGet(getURL string) ([]byte, error) {
	cmd := exec.Command("az", "rest",
		"--method", "get",
		"--url", getURL,
		"--resource", azureDevOpsResourceID,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("az rest: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
