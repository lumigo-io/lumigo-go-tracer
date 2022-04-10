![Version](https://img.shields.io/badge/version-1.0.0-green.svg)
[![CircleCI](https://circleci.com/gh/lumigo-io/<REPOSITORY_NAME>/tree/master.svg?style=svg&circle-token=<CIRCLECI_TOKEN_PLACEHOLDER>)](https://circleci.com/gh/lumigo-io/<REPOSITORY_NAME>/tree/master)
[![codecov](https://codecov.io/gh/lumigo-io/<REPOSITORY_NAME>/branch/master/graph/badge.svg?token=<COVECOV_TOKEN_PLACEHOLDER>)](https://codecov.io/gh/lumigo-io/<REPOSITORY_NAME>)


# Prepare your machine
* Make sure your AWS environment is ready, please follow [this link](https://github.com/lumigo-io/welcome/wiki/Get-ready-to-AWS) on how to create your aws environment.
* Create a virtualenv `virtualenv venv -p python3`
* Activate the virtualenv by running `. venv/bin/activate`
* Run `pip install -r requirements.txt` to install dependencies.
* If you use pycharm, make sure to change its virtualenv through the PyCharm -> Preferences -> Project -> Interpreter under the menu
* Run `pre-commit install` in your repository to install pre-commits hooks.

# Deployment
* Run locally `./scripts/deploy.sh`.
* To redeploy just run `./scripts/deploy.sh` again.
* To remove the deployment just run `./scripts/remove.sh`.

# Testing
* Run `pytest` in the root folder.
