# Boilerplate
Use this boilerplate project to create a new python backend project. You can use some of its files also for other types of projects.

## Initial Setup
* Use `Use this template` to clone a new repo.
* In the `scripts/ci_deploy.sh` file, in the  `../utils/common_bash/defaults/ci_deploy.sh <REPO_NAME_PLACEHOLDER>` - replace the placeholder with the repo name
* Add your service to the uber script, example: https://github.com/lumigo-io/utils/commit/109024a0a0515c13a6a8703242a6a50e187f4de9
* In the `scripts/deploy.sh` file, change the printed service name, from: 
```
../utils/common_bash/defaults/deploy.sh "NAME OF YOUR SERVICE" $*"
```  
to reflect your service's name/functionality
* In the `scripts/remove.sh` file, change the printed service name, from: 
```
../utils/common_bash/defaults/remove.sh  "NAME OF YOUR SERVICE" $*
```  
to reflect your service's name/functionality

## Create the README.md of your new repo
* Replace the new repo's README.md with `README-TEMPLATE.md` (i.e. delete the new file and rename the template)
* You should merge your changes before starting "CircleCi Configuration"

## CircleCi Configuration
* Goto https://circleci.com/add-projects/gh/lumigo-io
* Click `Set Up Project` in your repository row
* Click on `Start Building` or `Add Config`
* Click on the settings ![image](https://user-images.githubusercontent.com/38886884/51681065-32691080-1fec-11e9-9e2d-4cdb116f75b8.png) 
* Click on `Checkout SSH keys`
* Click on `Authorize With Github`
* Click `Create and add <Githhub Username> user key`
* Go to CircleCi Settings -> Environment Variables
* Click `Import Variables` and from the drop-down list choose `tracing-ingestion` project. From the list of variables choose to import **only** the `KEY` and `DOCKERHUB_PASSWORD` variable (You might need to follow the project first).
* Go to Settings -> Advanced Settings
* Change `Auto-cancel redundant builds` to `on`
## Create a CircleCi badge (note - this process needs to be fixed)
* Click on `Status Badges`
* Set the `Branch` to master
* For API Token: Create a new token, name it `status`
* Copy the token of the new `status` token to the readme instead of the placeholder in the second code line

## Codecov Configuration
* Goto https://codecov.io/gh/lumigo-io/REPOSITORY_NAME
* You might need to log in or allow access
* Click on `Add new repository`
* Click on `Copy` to copy the codecov_token
* Goto CircleCi Settings -> Environment Variables
* Click `Add Variable`. Call it `CODECOV_TOKEN`, and add the value from you copied from codecov
* In codecove.io go to the Settings tab
* In the left menu click on `Badge`
* Click `Copy` in the Markdown section and paste to the codecov line in the readme file
(notice you will have to trim it a bit to look like the template line)

________
At this point, you should introduce your code, and run the tests via CircleCI successfully.
This is **mandatory** for the next step.
________


## Protect you repository
* In your repository goto `Settings` `Manage access`
* In `Manage access` click `Add teams` and select `Dev`. Click `Add lumigo-io/dev to <repo-name>`.
* Set `Write` permissions
* In `Branches`, click `Add rule`. And in `Branch name pattern` enter `master`
* In `Protect matching branches` check `Require pull request reviews before merging`
* Check `Require review from Code Owners`
* Check `Require status checks to pass before merging`
* Check `Require branches to be up to date before merging`
* Check `ci/circleci:test` 
* Check `ci/circleci:integration-test`
* Click `Create`
* 

## Code changes
* Add your new repository to `get_repository_enum` in `utils` repository under `common_bash/functions.sh`
* Add your new repository to `DEPLOY_EXECUTION_ORDER` & `PYTHON_REPOSITORIES` in `utils` repository under `deployment/sls_deploy/main.py`
* Make sure to add the team this repository belongs to to `serverless.yml`. See https://github.com/lumigo-io/backend-python-boilerplate/blob/67f5609d3c2c96a2d5f63b7f46033b66428ba9b4/create_aws_resources/serverless-TEMPLATE.yml#L10
* Change team ownership under https://github.com/lumigo-io/backend-python-boilerplate/blob/master/.github/CODEOWNERS
