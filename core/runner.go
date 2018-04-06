// Copyright © 2018 Tobias Jeske
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/blang/semver"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// load an app file, process it and return its content as a struct
func getProcessedAppConfig(appFile string) yamlSpec {
	absContainerFlightFile, err := filepath.Abs(appFile)
	checkErr(err)

	env := getEnv(absContainerFlightFile)

	config := getAppConfig(env)

	// check version
	parsedRange, err := semver.ParseRange(config.Version)
	checkErrMsg(err, "Version information must match semver 2.0.0 (https://semver.org/)!")
	if !parsedRange(ContainerFlightVersion()) {
		log.Fatal("App file is not compatible with current Container Flight version " + ContainerFlightVersion().String() + "!")

	}

	return config
}

// PrintDockerfile loads an app file and dump the processed dockerfile
func PrintDockerfile(appFile string) {

	config := getProcessedAppConfig(appFile)

	fmt.Println(config.Docker.Dockerfile)
}

// Run starts an app in a container.
// If the container does not exists it is built upfront.
func Run(args []string) {

	appFile := args[0]

	config := getProcessedAppConfig(appFile)

	var httpClient *http.Client
	cli, err := client.NewClient(client.DefaultDockerHost, "1.30", httpClient, nil)
	checkErr(err)

	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	checkErr(err)

	containerLabel := getDockerContainerLabel(appFile, config.Docker.Dockerfile)
	fullContainerLabel := containerLabel + ":latest"

	found := false
	for _, image := range images {
		for _, repoTag := range image.RepoTags {
			if repoTag == fullContainerLabel {
				found = true
			}
		}
	}

	dockerClient := newDockerClient(containerLabel)
	if !found {
		dockerClient.build(&config.Docker.Dockerfile)
	}

	// set hostname if the user has not specified it
	additionalDockerArgs := []string{"-h", "flybydocker"}
	runArgs := config.Docker.RunArgs
	for i := 0; i < len(runArgs); i++ {
		arg := runArgs[i]
		if arg == "-h" {
			additionalDockerArgs = []string{}
			break
		}
	}
	var runCmd []string

	if config.Console {
		runCmd = []string{
			"-ti",
			"-a", "stdin",
			"-a", "stdout",
		}
	}

	if config.Gui {
		runCmd = []string{
			"-e", "DISPLAY=" + os.Getenv("DISPLAY"),
			"-v", "/tmp/.X11-unix:/tmp/.X11-unix",
		}
	}

	runCmd = append(runCmd, additionalDockerArgs...)
	runCmd = append(runCmd, runArgs...)
	runCmd = append(runCmd, containerLabel)
	runCmd = append(runCmd, args[1:]...)

	dockerClient.run(&runCmd)
}
