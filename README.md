## github-actions-watcher

### Goal
For a project with flaky tests, watching for failures and restarting those tests is time consuming.
`github-actions-watcher` watches CI Actions on Pull Requests in a Github Project and restarts failed workflows (up to a max of 4 attempts). 

### Usage

Setup a personal access (or similar) token in Github for your repo.
Specify that, the github organization name, repository name you want to watch and (for now) the PR you want to restart tests for.


`go run main.go -pr <pr_number> -token <access_token> -org <org_name> -repo <repo_name>`


### Status
_*In Development*_

Takes a PR number and restarts related workflows.

Next Steps:
* Convert to long-running process which wakes up every <N> seconds, watches all PRs, created within <X> days and restarts failed tests
* add label TooManyFailures
* Log failed tests and retries in a PlanetScale DB
* Vercel UI to display above data

Figure out:
* Create cron job to run the watcher. Options:
  * as a CRON workflow in Actions. Con: token will be publicly visible 
  * Serverless function in private cloud
  

