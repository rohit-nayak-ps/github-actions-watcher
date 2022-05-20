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
	"fmt"
	"github.com/google/go-github/v44/github"
	"golang.org/x/oauth2"
	"os"
	"strings"
	"time"
)

const (
	maxWorkflowRetryAttempts = 3  // we will not attempt to retry workflows which have failed these many times
	maxPRAgeDays             = 3  // only look for PRs updated within these days
	maxAllowedFailures       = 10 // if a test has > these failures, assume it is genuinely failing CI
	maxPRsToProcessAtATime   = 10 // as a throttling measure, only these many eligible workflows will be tried
)

func setup(logFile *os.File) {

	setupLogging(logFile)
	getOptions()
}

func main() {

	var err error
	var w *os.File
	w, err = os.OpenFile(logFileName, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer w.Close()
	setup(w)

	ctx := context.Background()
	client, err := getClient(ctx)
	if err != nil {
		panic(err.Error())
	}

	if watcherConfig.isDryRun {
		prn.Printf("***** START DRY RUN *****")
		defer func() {
			prn.Printf("***** END DRY RUN *****")
		}()
	} else {
		if watcherConfig.psDSN != "" {
			psdb, err = initPlanetScale(ctx)
			if err != nil {
				prn.Printf(err.Error())
				os.Exit(-1)
			}
		}
	}

	prn.Printf("***** Starting watcher for %s/%s", watcherConfig.githubOrg, watcherConfig.githubRepo)

	if watcherConfig.optPRNumber != 0 { // specific pr specified, we only process that one
		dbg.Printf("Processing specified PR %d", watcherConfig.optPRNumber)
		prn.Printf("Processing specified PR %d", watcherConfig.optPRNumber)
		numStarted := processPR(ctx, client, watcherConfig.optPRNumber)
		prn.Printf("Started %d workflow(s) for pr %d", numStarted, watcherConfig.optPRNumber)
		dbg.Println("DONE")
		return
	}

	dbg.Printf("Processing recent open PRs")
	prn.Printf("Processing recent open PRs")
	prs, err := getPRsToProcess(ctx, client)
	prn.Printf("Found %d recent open PRs", len(prs))
	prsProcessed := 0
	for _, pr := range prs {
		dbg.Printf("Processing PR %d", *pr.Number)
		prn.Printf("Processing PR %d", *pr.Number)
		numStarted := processPR(ctx, client, *pr.Number)
		prn.Printf("Started %d workflows for pr %d", numStarted, *pr.Number)
		if numStarted > 0 {
			prsProcessed++
		}
		if prsProcessed > maxPRsToProcessAtATime {
			prn.Printf("reached max limit of PRs to process: %d", prsProcessed)
			break
		}
	}
	prn.Printf("Done")
}

func getPRsToProcess(ctx context.Context, client *github.Client) ([]*github.PullRequest, error) {
	var prsToProcess []*github.PullRequest
	opts := &github.PullRequestListOptions{
		State:       "open",
		Sort:        "updated",
		Direction:   "desc",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	prs, _, err := client.PullRequests.List(ctx, watcherConfig.githubOrg, watcherConfig.githubRepo, opts)
	if err != nil {
		return nil, err
	}
	for _, pr := range prs {
		// can do better?: can't differentiate b/w PRs blocked because tests have not passed OR not yet reviewed ...
		if pr.MergeableState != nil && *pr.MergeableState != "blocked" {
			continue
		}
		updatedAt := *pr.UpdatedAt
		if time.Since(updatedAt).Hours() < 24*maxPRAgeDays {
			prsToProcess = append(prsToProcess, pr)
			if len(prsToProcess) >= maxPRsToProcessAtATime {
				continue
			}
		}
	}
	return prsToProcess, nil
}

func processPR(ctx context.Context, client *github.Client, prNumber int) int {
	pr := getPR(ctx, client, prNumber)

	if pr == nil {
		dbg.Printf("pr %d not found", prNumber)
		return 0
	}
	head := *pr.Head.SHA
	branch := strings.Split(*pr.Head.Label, ":")[1]
	if numStarted, err := restartFailedActions(ctx, client, prNumber, head, branch); err != nil {
		dbg.Printf("no pr id specified")
		return 0
	} else {
		return numStarted
	}
}

func getPR(ctx context.Context, client *github.Client, prNumber int) *github.PullRequest {
	pr, state, err := client.PullRequests.Get(ctx, watcherConfig.githubOrg, watcherConfig.githubRepo, int(prNumber))
	if err != nil {
		dbg.Printf("getPR err: %s", err)
		return nil
	}
	dbg.Printf("PR %d:  MergeableState: %s, State: %s, SHA %s", *pr.Number, *pr.MergeableState, *pr.State, *pr.Head.SHA)
	dbg.Printf("Client token and pagination state: %+v", state)
	return pr
}

func restartFailedActions(ctx context.Context, client *github.Client, prNumber int, sha, branch string) (numStarted int, err error) {
	numStarted = 0
	status := "latest"
	completed := "completed"
	opts := &github.ListCheckRunsOptions{
		CheckName: nil,
		Status:    &completed,
		Filter:    &status,
		AppID:     nil,
		ListOptions: github.ListOptions{
			PerPage: 200,
		},
	}
	cs, _, err := client.Checks.ListCheckRunsForRef(ctx, watcherConfig.githubOrg, watcherConfig.githubRepo, sha, opts)
	if err != nil {
		return 0, fmt.Errorf("GetCheckSuite err: %s", err)
	}
	numFailures := 0
	for _, checkRun := range cs.CheckRuns {
		if *checkRun.Conclusion == "failure" {
			numFailures++
		}
	}
	if numFailures > maxAllowedFailures {
		prn.Printf("Too many failures for PR %d, not attempting to retry any tests", prNumber)
		dbg.Printf("Too many failures for PR %d, not attempting to retry any tests", prNumber)
		return 0, nil
	}
	for i, checkRun := range cs.CheckRuns {
		if *checkRun.Conclusion != "failure" {
			continue
		}
		dbg.Printf("%d: Name: %s  Status:%s, Conclusion:%s, ID %d", i, *checkRun.Name, *checkRun.Status, *checkRun.Conclusion, *checkRun.ID)
		wfRun := getWorkflowRun(ctx, client, branch, *checkRun.ID, *checkRun.Name)
		if wfRun == nil {
			dbg.Printf("No Workflow Run found")
			continue
		}
		if *wfRun.RunAttempt >= maxWorkflowRetryAttempts {
			dbg.Printf("Not attempting to rerun %s since it has already been run %d times", *wfRun.Name, *wfRun.RunAttempt)
			continue
		}
		if psdb != nil {
			sqlInsertFailedAction := "insert into action(pr, workflow, attempt, check_suite_url) values (?, ?, ?, ?);"
			if _, err := psdb.Exec(sqlInsertFailedAction, prNumber, *wfRun.Name, *wfRun.RunAttempt, *wfRun.HTMLURL); err != nil {
				dbg.Printf("Error inserting into psdb: %s", err)
				prn.Printf("Error inserting into psdb: %s", err)
			}
		}
		if rerunFailedJob(ctx, client, int(*wfRun.ID)) != nil {
			dbg.Printf("Failed starting workflowId %s", *wfRun.Name)
		} else {
			numStarted++
			dbg.Printf("Successfully started workflowId %s", *wfRun.Name)
		}
	}
	return numStarted, nil
}

func getClient(ctx context.Context) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: watcherConfig.githubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return client, nil
}

func getWorkflowRun(ctx context.Context, client *github.Client, branch string, checkSuiteID int64, wfName string) *github.WorkflowRun {
	workflowOptions := &github.ListWorkflowRunsOptions{
		Branch:      branch,
		ListOptions: github.ListOptions{PerPage: 1000},
	}

	wfRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, watcherConfig.githubOrg, watcherConfig.githubRepo, workflowOptions)
	if err != nil {
		dbg.Printf("getWorkflowRun err: %s", err)
		return nil
	}
	//dbg.Printf("Got %d wfRuns", len(wfRuns.WorkflowRuns))

	completedJobs := make(map[string]bool)
	for _, wfRun := range wfRuns.WorkflowRuns {
		if *wfRun.Status != "completed" || (wfRun.Conclusion == nil || *wfRun.Conclusion != "failure") {
			continue
		}
		//dbg.Printf("Status %s, Conclusion %s", *wfRun.Status, *wfRun.Conclusion)
		wfJobs, _, err := client.Actions.ListWorkflowJobs(ctx, watcherConfig.githubOrg, watcherConfig.githubRepo, *wfRun.ID, nil)
		if err != nil {
			dbg.Printf("ListWorkflowJobs err: %s", err)
			return nil
		}
		for _, wfJob := range wfJobs.Jobs {
			jobName := *wfJob.Name
			//dbg.Printf("wfName=%s, *wfRun.Name=%s, job name=%s", wfName, *wfRun.Name, jobName )
			if *wfJob.Status != "completed" {
				continue
			}
			if wfName == *wfJob.Name || *wfRun.CheckSuiteID == checkSuiteID {
				switch *wfJob.Conclusion {
				case "success":
					completedJobs[jobName] = true
				case "failure":
					if _, ok := completedJobs[jobName]; !ok {
						dbg.Printf("Found wfName=%s, *wfRun.Name=%s, %s,%s,%s", wfName, *wfRun.Name, *wfJob.Conclusion, *wfJob.Status, *wfJob.CompletedAt)
						return wfRun
					}
				}
			} else {
				continue
			}
		}

	}
	return nil
}

func rerunFailedJob(ctx context.Context, client *github.Client, wfId int) error {
	if watcherConfig.isDryRun {
		return nil
	}
	_, err := client.Actions.RerunFailedJobsByID(ctx, watcherConfig.githubOrg, watcherConfig.githubRepo, int64(wfId))
	if err != nil {
		dbg.Printf("RerunWorkflowByID err: %s", err)
		return err
	}
	return nil
}
