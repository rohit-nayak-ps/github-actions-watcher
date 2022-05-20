## github-actions-watcher

_Temporarily hosting this in my personal repo_

### What it does

For a project with lots of flaky tests, watching for failures and restarting those tests is time consuming.
`github-actions-watcher` watches CI Actions on Pull Requests in a Github Project and restarts failed workflows.

PRs considered for retry are:

* Single PR is specified on command line
* Default Most recent (updated < 3days) open PRs.
* If any PR has a workflow with too many failing workflows (> 10) it will ignore it.
* Workflows which have already run too many (>=3) times are ignored.
* There is an upper limit (10) of PRs processed at a time, as a throttling measure

Important logs output to `stderr` , with more detailed logging in `./watcher.log`.

It is possible to record the detected failed workflows in a `PlanetScale` database. All you need to do is to specify your
`PlanetScale DSN`.

There is a companion UI in progress which uses Vercel to display the data stored in PlanetScale, current version
at https://github-actions-watcher-ui.vercel.app/. 

### Installation/Usage

##### Installation

```
git clone git@github.com:rohit-nayak-ps/github-actions-watcher.git
make
```

##### Prerequisite

Setup a personal access (or similar) token in Github for your repo.

#### Usage

Specify the token, the github organization name, repository name you want to watch and, optionally the PR, you want to
restart tests for.

##### Usage

`github-actions-watcher -token <access_token> -org vitessio -repo vitess [-dryrun] [-pr <pr_number>]`

```
Usage of ./github-actions-watcher:
  -dryrun
    	Just log what we will do, no failed actions will be restarted
  -org string
    	Github Organization Name
  -pr int
    	(Optional) Github PR# to process, default: top N PRs
  -pstoken string
    	(Optional) PlanetScale Token, if specified, records detected failures
  -repo string
    	Github Repository Name
  -token string
    	Github Personal Access Token

```

###### Sample Usage

```
rohit@rohit-ubuntu:~/github-actions-watcher$ ./github-actions-watcher  -dryrun -org vitessio -repo vitess -token ghp_redacted -pr 10335
2022/05/19 13:10:19 ***** START DRY RUN *****
2022/05/19 13:10:19 ***** Starting watcher for vitessio/vitess
2022/05/19 13:10:19 Processing specified PR 10335
2022/05/19 13:10:22 Too many failures for PR 10335, not attempting to retry any tests
2022/05/19 13:10:22 Started 0 workflow(s) for pr 10335
2022/05/19 13:10:22 ***** END DRY RUN *****

rohit@rohit-ubuntu:~/github-actions-watcher$ ./github-actions-watcher  -dryrun -org vitessio -repo vitess -token ghp_redacted 
2022/05/19 13:13:14 ***** START DRY RUN *****
2022/05/19 13:13:14 ***** Starting watcher for vitessio/vitess
2022/05/19 13:13:14 Processing recent open PRs
2022/05/19 13:13:17 Found 13 recent open PRs
2022/05/19 13:13:17 Processing PR 10306
2022/05/19 13:13:20 Too many failures for PR 10306, not attempting to retry any tests
2022/05/19 13:13:20 Started 0 workflows for pr 10306
...
...

```

### Status

_*In Development*_

#### ToDos

* add label TooManyFailures
* Create cron job to run the watcher. Options:
    * as a CRON workflow in Actions. Con: token will be publicly visible
    * Serverless function in private cloud
* Use PlanetScale service tokens instead of the DSN
* Aggregate recent failures

