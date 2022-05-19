## github-actions-watcher

_Temporarily hosting this in my personal repo_

### Goal
For a project with flaky tests, watching for failures and restarting those tests is time consuming.
`github-actions-watcher` watches CI Actions on Pull Requests in a Github Project and restarts failed workflows (up to a max of 4 attempts). 

### Installation/Usage

##### Installation
```
git clone git@github.com:rohit-nayak-ps/github-actions-watcher.git
make
```

##### Prerequisite

Setup a personal access (or similar) token in Github for your repo.

#### Usage

Specify the token, the github organization name, repository name you want to watch and, optionally the PR, you want to restart tests for.

##### Sample Usage
`github-actions-watcher -token <access_token> -org vitessio -repo vitess [-dryrun] [-pr <pr_number>]`

##### Options
```
Usage of ./github-actions-watcher:
  -dryrun
    	Just log what we will do, no failed actions will be restarted
  -org string
    	Github Organization Name
  -pr int
    	Github PR#
  -repo string
    	Github Repository Name
  -token string
    	Github Personal Access Token

```

### Status
_*In Development*_

Takes a PR number and restarts related workflows.

Required TBD:
* Convert to long-running process which wakes up every <N> seconds, watches all PRs, created within <X> days and restarts failed tests
* add label TooManyFailures
* Create cron job to run the watcher. Options:
  * as a CRON workflow in Actions. Con: token will be publicly visible
  * Serverless function in private cloud

Nice-to-have TBD:
* Log failed tests and retries in PlanetScale
* Vercel UI to display above data
* Aggregate recent failures

### Issues
* Need to check if I was able to do all this using a Personal Access token because I am a maintainer, and there are ACLs in effect in the Github API or if we need to configure the same at our repo level. ...
It says here https://docs.github.com/en/developers/github-marketplace/creating-apps-for-github-marketplace/security-best-practices-for-apps that personal tokens should not be allowed in Apps, but that is what I am precisely doing ... 


