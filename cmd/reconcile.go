/*
Copyright 2024 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/obscli/types"
	"sigs.k8s.io/release-sdk/obs"
	"sigs.k8s.io/yaml"
)

type Options struct {
	ManifestPath string
	OBSClient    *obs.OBS
}

type Info struct {
	Username  string
	Password  string
	APIURL    string
	OBSClient *obs.OBS
}

const DefaultAPIURL = "https://api.opensuse.org/"

func Reconcile() *cobra.Command {
	var (
		opts   Options
		apiURL string
	)

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "reconcile command for Paketo",
		Run: func(cmd *cobra.Command, args []string) {
			cred, err := GetOBSCredentials(apiURL)
			if err != nil {
				log.Fatalf("Error getting OBS credentials: %v\n", err)
			}

			opts.OBSClient = cred.OBSClient

			opts.ManifestPath, _ = cmd.Flags().GetString("manifest")
			if !opts.CheckManifestPath() {
				fmt.Printf("%s does not exist\n", opts.ManifestPath)
				return
			}

			prjs, err := LoadManifest(opts.ManifestPath)
			if err != nil {
				fmt.Printf("%v\n", err)
				return
			}

			for _, prj := range prjs.Projects {
				remotePrj, err := opts.OBSClient.GetProjectMetaFile(context.Background(), prj.Name)
				if err != nil {
					fmt.Printf("error getting project from OBS: %v\n", err)
					continue
				}

				if different := compareProjects(prj, remotePrj); different {
					err := opts.OBSClient.CreateUpdateProject(context.Background(), &prj.Project)
					if err != nil {
						fmt.Printf("error creating/updating project on OBS: %v\n", err)
					} else {
						fmt.Printf("Project %s updated on OBS.\n", prj.Name)
					}
				} else {
					fmt.Printf("Project %s is already up-to-date.\n", prj.Name)
				}
			}
		},
	}

	cmd.Flags().StringVarP(&opts.ManifestPath, "manifest", "m", "", "Specify the path to read the example manifest")
	cmd.MarkFlagRequired("manifest")
	cmd.Flags().StringVar(&apiURL, "api-url", DefaultAPIURL, "The base URL for the API")

	return cmd
}

func compareProjects(local types.Project, remote *obs.Project) bool {
	if local.Name != remote.Name ||
		local.Title != remote.Title ||
		local.Description != remote.Description ||
		local.URL != remote.URL ||
		!comparePersons(local.Persons, remote.Persons) ||
		!compareRepositories(local.Repositories, remote.Repositories) {
		return true
	}
	return false
}

func comparePersons(localPersons, remotePersons []obs.Person) bool {
	if len(localPersons) != len(remotePersons) {
		return false
	}

	for i, localPerson := range localPersons {
		if localPerson.UserID != remotePersons[i].UserID || localPerson.Role != remotePersons[i].Role {
			return false
		}
	}

	return true
}

func compareRepositories(localRepos, remoteRepos []obs.Repository) bool {
	if len(localRepos) != len(remoteRepos) {
		return false
	}

	for i, localRepo := range localRepos {
		if localRepo.Repository != remoteRepos[i].Repository ||
			!compareArchitectures(localRepo.Architectures, remoteRepos[i].Architectures) {
			return false
		}
	}

	return true
}

func compareArchitectures(localArchs, remoteArchs []string) bool {
	if len(localArchs) != len(remoteArchs) {
		return false
	}

	for i, localArch := range localArchs {
		if localArch != remoteArchs[i] {
			return false
		}
	}

	return true
}

func GetOBSCredentials(apiURL string) (*Info, error) {
	username := os.Getenv("OBS_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("OBS_USERNAME environment variable not set")
	}

	password := os.Getenv("OBS_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("OBS_PASSWORD environment variable not set")
	}

	// Initialize OBS client using provided credentials
	obsClient := obs.New(&obs.Options{
		Username: username,
		Password: password,
		APIURL:   apiURL,
	})

	// Return OBS client along with other credentials
	return &Info{
		Username:  username,
		Password:  password,
		APIURL:    apiURL,
		OBSClient: obsClient,
	}, nil
}

func (o *Options) CheckManifestPath() bool {
	_, err := os.Stat(o.ManifestPath)
	return err == nil
}

func LoadManifest(path string) (*types.Projects, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("unable to read the manifest content: %v", err)
	}

	var prjs types.Projects
	if err := yaml.Unmarshal(bytes, &prjs); err != nil {
		return nil, fmt.Errorf("error unmarshalling yaml: %v", err)
	}

	return &prjs, nil
}
