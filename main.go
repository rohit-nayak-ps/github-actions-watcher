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
	"flag"
	"fmt"
	"github.com/google/go-github/v44/github"
	"golang.org/x/oauth2"
	"log"
	"os"
	"strings"
)

const (
	maxWorkflowRetryAttempts = 4
)

var githubOrg, githubRepo, githubToken string
var prID int

func getOptions() {
	id := flag.Int("pr", 0, "Github PR#")
	org := flag.String("org", "", "Github Organization Name")
	repo := flag.String("repo", "", "Github Repository Name")
	token := flag.String("token", "", "Github Personal Access Token")

	flag.Parse()

	if *id == 0 || *org == "" || *repo == "" || *token == "" {
		flag.Usage()
		os.Exit(-1)
	}

	githubOrg = *org
	githubRepo = *repo
	githubToken = *token
	prID = *id
}
func main() {
	var err error
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Ldate)
	getOptions()

	ctx := context.Background()
	client, err := getClient(ctx)
	if err != nil {
		log.Println(err.Error())
		os.Exit(-1)
	}

	fmt.Printf("\n\n***** Starting watcher for %s/%s, PR %d\n\n\n", githubOrg, githubRepo, prID)
	pr := getPR(ctx, client, prID)

	if pr == nil {
		log.Printf("pr %d not found", prID)
		os.Exit(-1)
	}
	head := *pr.Head.SHA
	branch := strings.Split(*pr.Head.Label, ":")[1]
	if numStarted, err := restartFailedActions(ctx, client, head, branch); err != nil {
		log.Printf("no pr id specified")
		os.Exit(-1)
	} else {
		fmt.Printf("\n\n***** Done, started %d workflows for pr %d *****\n\n", numStarted, pr.Number)
	}
}

func getPR(ctx context.Context, client *github.Client, prId int) *github.PullRequest {
	pr, state, err := client.PullRequests.Get(ctx, githubOrg, githubRepo, prId)
	if err != nil {
		log.Printf("getPR err: %s", err)
		return nil
	}
	log.Printf("PR %d:  MergeableState: %s, State: %s, SHA %s", *pr.Number, *pr.MergeableState, *pr.State, *pr.Head.SHA)
	log.Printf("Client token and pagination state: %+v", state)
	return pr
}

func restartFailedActions(ctx context.Context, client *github.Client, sha, branch string) (int, error) {
	numStarted := 0
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
	cs, _, err := client.Checks.ListCheckRunsForRef(ctx, githubOrg, githubRepo, sha, opts)
	if err != nil {
		return 0, fmt.Errorf("GetCheckSuite err: %s", err)
	}
	//log.Printf("Total Runs: %d", *cs.Total)
	for i, checkRun := range cs.CheckRuns {
		if *checkRun.Conclusion != "failure" {
			continue
		}
		log.Printf("%d: Name: %s  Status:%s, Conclusion:%s, ID %d", i, *checkRun.Name, *checkRun.Status, *checkRun.Conclusion, *checkRun.ID)
		wfRun := getWorkflowRun(ctx, client, branch, *checkRun.ID, *checkRun.Name)
		if wfRun == nil {
			log.Printf("No Workflow Run found")
			continue
		}
		if *wfRun.RunAttempt >= maxWorkflowRetryAttempts {
			log.Printf("Not attempting to rerun %s since it has already been run %d times", *wfRun.Name, *wfRun.RunAttempt)
			continue
		}
		if rerunFailedJob(ctx, client, int(*wfRun.ID)) != nil {
			log.Printf("Failed starting workflowId %s", *wfRun.Name)
		} else {
			numStarted++
			log.Printf("Successfully started workflowId %s", *wfRun.Name)
		}
	}
	return numStarted, nil
}

func getClient(ctx context.Context) (*github.Client, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubToken},
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

	wfRuns, _, err := client.Actions.ListRepositoryWorkflowRuns(ctx, githubOrg, githubRepo, workflowOptions)
	if err != nil {
		log.Printf("getWorkflowRun err: %s", err)
		return nil
	}
	//log.Printf("Got %d wfRuns", len(wfRuns.WorkflowRuns))

	completedJobs := make(map[string]bool)
	for _, wfRun := range wfRuns.WorkflowRuns {
		if *wfRun.Status != "completed" || (wfRun.Conclusion == nil || *wfRun.Conclusion != "failure") {
			continue
		}
		//log.Printf("Status %s, Conclusion %s", *wfRun.Status, *wfRun.Conclusion)
		wfJobs, _, err := client.Actions.ListWorkflowJobs(ctx, githubOrg, githubRepo, *wfRun.ID, nil)
		if err != nil {
			log.Printf("ListWorkflowJobs err: %s", err)
			return nil
		}
		for _, wfJob := range wfJobs.Jobs {
			jobName := *wfJob.Name
			//log.Printf("wfName=%s, *wfRun.Name=%s, job name=%s", wfName, *wfRun.Name, jobName )
			if *wfJob.Status != "completed" {
				continue
			}
			if wfName == *wfJob.Name || *wfRun.CheckSuiteID == checkSuiteID {
				switch *wfJob.Conclusion {
				case "success":
					completedJobs[jobName] = true
				case "failure":
					if _, ok := completedJobs[jobName]; !ok {
						log.Printf("Found wfName=%s, *wfRun.Name=%s, %s,%s,%s", wfName, *wfRun.Name, *wfJob.Conclusion, *wfJob.Status, *wfJob.CompletedAt)
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
	_, err := client.Actions.RerunFailedJobsByID(ctx, githubOrg, githubRepo, int64(wfId))
	if err != nil {
		log.Printf("RerunWorkflowByID err: %s", err)
		return err
	}
	return nil
}
