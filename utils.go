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
	"context"
	"database/sql"
	"flag"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
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
var psdb *sql.DB

type config struct {
	githubOrg, githubRepo, githubToken string
	isDryRun                           bool
	optPRNumber                        int
	psDSN                              string
	ignoreWorkflows                    []string
}

// process command line options and set global variables for use
func getOptions() {
	org := flag.String("org", "", "Github Organization Name")
	repo := flag.String("repo", "", "Github Repository Name")
	token := flag.String("token", "", "Github Personal Access Token")
	dryrun := flag.Bool("dryrun", false, "Just log what we will do, no failed actions will be restarted")
	psDSN := flag.String("pstoken", "", "(Optional) PlanetScale Token, if specified, records detected failures")
	prNumber := flag.Int("pr", 0, "(Optional) Github PR# to process, default: top N PRs")
	ignoreWorkflowList := flag.String("ignore", "", "CSV of workflow name substrings to ignore")

	flag.Parse()

	if *org == "" || *repo == "" || *token == "" {
		flag.Usage()
		os.Exit(-1)
	}

	var ignoreWorkflows []string
	if *ignoreWorkflowList != "" {
		arr := strings.Split(strings.TrimSpace(*ignoreWorkflowList), ",")
		for _, wf := range arr {
			ignoreWorkflows = append(ignoreWorkflows, strings.ToLower(strings.TrimSpace(wf)))
		}
	}
	watcherConfig = &config{
		githubOrg:       *org,
		githubRepo:      *repo,
		githubToken:     *token,
		optPRNumber:     *prNumber,
		isDryRun:        *dryrun,
		psDSN:           *psDSN,
		ignoreWorkflows: ignoreWorkflows,
	}
}

func initPlanetScale(ctx context.Context) (*sql.DB, error) {
	var err error
	db, err := sql.Open("mysql", watcherConfig.psDSN)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}
