/*
 * Copyright 2022 The Vitess Authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"io"
	"log"
	"os"
)

var logFileName string
var dbg *log.Logger
var prn *log.Logger

func init() {
	logFileName = "./watcher.log"
}

func setupLogging(w io.Writer) {
	dbg = log.New(w, "", log.LstdFlags|log.Lshortfile|log.Ldate)
	prn = log.New(os.Stderr, "", log.LstdFlags|log.Ldate)
}

var watcherConfig *config

type config struct {
	githubOrg, githubRepo, githubToken string
	optPRNumber                        int
	isDryRun                           bool
}

// process command line options and set global variables for use
func getOptions() {
	prNumber := flag.Int("pr", 0, "Github PR#")
	org := flag.String("org", "", "Github Organization Name")
	repo := flag.String("repo", "", "Github Repository Name")
	token := flag.String("token", "", "Github Personal Access Token")
	dryrun := flag.Bool("dryrun", false, "Just log what we will do")

	flag.Parse()

	if *org == "" || *repo == "" || *token == "" {
		flag.Usage()
		panic("")
	}
	watcherConfig = &config{
		githubOrg:   *org,
		githubRepo:  *repo,
		githubToken: *token,
		optPRNumber: *prNumber,
		isDryRun:    *dryrun,
	}
}
