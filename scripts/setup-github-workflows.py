#!/usr/bin/env python
# SPDX-License-Identifier: Apache-2.0
# Copyright Red Hat, Inc.

import argparse
import json
import os
import shutil
import subprocess
import tempfile

from git import GitCommandError
from git.repo import Repo
from pathlib import Path
from urllib.parse import urljoin


GITHUB_API = "https://api.github.com"
GITHUB_USERNAME = os.getenv('USERNAME')
# GitHub PAT is needed to run this script, and GitHub recommends that you use a
# fine-grained personal access token instead of a personal access token (classic)
# whenever possible.
# Ref: https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api?apiVersion=2022-11-28
# The mininal required permissions for a find-grained token in this script include:
# - Actions: read-only
# - Administration: read and write
# - Contents: read and write
# - Metadata: read-only
# - Pull requests: read and write
# - Workflows: read and write
GITHUB_TOKEN = os.getenv('GITHUB_PAT')

# The workflow is identified by the workflow filename or ID
WORKFLOWS_PATH = '.github/workflows'
COMMON_WORKFLOWS = ['codeql', 'scorecard', 'sonarcloud']


def parse_args():
    parser = argparse.ArgumentParser(description="Check repo workflows")
    parser.add_argument(
        "repo_url",
        nargs=1,
        help="GitHub repository URL (e.g., https://github.com/owner/repo)",
    )
    args = parser.parse_args()
    return args


def extract_owner_repo(repo_url):
    if repo_url.endswith(".git"):
        repo_url = repo_url[:-4]
    parts = repo_url.split("/")
    owner = parts[-2]
    repo = parts[-1]
    return owner, repo


def copy_workflow_file(workflow_filename, dest_project_path):
    """
    Copies a specified workflow file to a destination project.

    Args:
        workflow_filename (str): The name of the workflow file to copy.
        dest_project_path (str): The path to the destination project directory.
    """

    # The source workflow files will be moved to a new repo
    src_workflows_dir = os.path.join(
            os.path.abspath(__file__ + "/../../"), WORKFLOWS_PATH
    )
    dest_workflows_dir = os.path.join(dest_project_path, WORKFLOWS_PATH)
    os.makedirs(dest_workflows_dir, exist_ok=True)
    src_file_path = os.path.join(src_workflows_dir, workflow_filename)
    dest_file_path = os.path.join(dest_workflows_dir, workflow_filename)

    if os.path.exists(dest_file_path):
        print(f"{workflow_filename} already exists in {dest_project_path}.")
        return False
    else:
        try:
            shutil.copy(src_file_path, dest_file_path)
        except FileNotFoundError:
            print(f"Error: Source file {src_file_path} not found.")
        except Exception as e:
            print(f"An error occurred while copying file {src_file_path}: {e}")
    return True


def push_to_origin(repo_path, workflow, owner, repo_name):
    workflow_branch = "auto-workflow-" + workflow
    repo = Repo(repo_path)

    print("Start to push updates to remote")
    try:
        repo.git.checkout("-b", workflow_branch)
    except GitCommandError as e:
        raise RepoException(f"Git checkout failed: {e}") from e
    repo.git.add(os.path.join(WORKFLOWS_PATH, workflow + '.yml'))

    commit_msg = f"ci: auto add {workflow} workflow file"
    repo.index.commit(commit_msg)
    remote = repo.remote()
    remote.set_url(f"https://{GITHUB_USERNAME}:{GITHUB_TOKEN}@github.com/{owner}/{repo_name}.git")
    remote.push(refspec=f"HEAD:{workflow_branch}", force=True)
    print("Done")
    return workflow_branch


def create_pull_request(workflow, owner, repo, workflow_branch):
    pulls_path = f'/repos/{owner}/{repo}/pulls'
    pulls_api = urljoin(GITHUB_API, pulls_path)
    cmd_header = '-H "Accept: application/vnd.github+json" ' \
                 f'-H "Authorization: Bearer {GITHUB_TOKEN}" ' \
                 '-H "X-GitHub-Api-Version: 2022-11-28"'
    data = {
            "title": f"ci: auto workflow update for {workflow}",
            "body": f"The update is for github {workflow} workflow.",
            "base": "main",
            "head": f"{owner}:{workflow_branch}"
    }
    cmd = f'curl -L -s -X POST {cmd_header} {pulls_api} -d \'{json.dumps(data)}\''

    print(f"Creating a pull request for {workflow} workflow update")
    try:
        subprocess.check_call(cmd, shell=True)
    except subprocess.CalledProcessError as e:
        raise RuntimeError(e)
    print("Done")


def check_workflows(repo_url):
    owner, repo = extract_owner_repo(repo_url)
    # Get all workflows and their states
    cmd_header = '-H "Accept: application/vnd.github+json" ' \
                 f'-H "Authorization: Bearer {GITHUB_TOKEN}" ' \
                 '-H "X-GitHub-Api-Version: 2022-11-28"'
    workflows_path = f'/repos/{owner}/{repo}/actions/workflows'
    workflows_api = urljoin(GITHUB_API, workflows_path)
    cmd = f'curl -L -s {cmd_header} {workflows_api}'
    output = subprocess.check_output(cmd, shell=True)

    workflows = json.loads(output).get('workflows')
    workflow_dict = {Path(w['path']).stem: w['state'] for w in workflows}
    print("Workflows: " + str(workflow_dict))

    for workflow in COMMON_WORKFLOWS:
        workflow_file = f'{workflow}.yml'
        if workflow in workflow_dict:
            if workflow_dict[workflow] != 'active':
                enable_endpoint = f'{workflows_api}/{workflow_file}/enable'
                cmd = f'curl -L -X PUT -s {cmd_header} {enable_endpoint}'
                print(f"Enabling {workflow}")
                try:
                    subprocess.check_call(cmd, shell=True)
                except subprocess.CalledProcessError as e:
                    raise RuntimeError(e)
                print("Done")

        # Otherwise, add the workflow file or enable with configuration
        else:
            # For CodeQL, enable the workflow with default setup
            if workflow == 'codeql':
                # For CodeQL, configure with default setup
                codeql_endpoint = f'{GITHUB_API}/repos/{owner}/{repo}/code-scanning/default-setup'
                data = '{"state":"configured"}'
                cmd = f'curl -L -X PATCH -s {cmd_header} {codeql_endpoint} -d \'{data}\''
                print(f"Configuring CodeQL: {cmd}")
                try:
                    subprocess.check_call(cmd, shell=True)
                except subprocess.CalledProcessError as e:
                    raise RuntimeError(e)
                print("Done")

            else:
                with tempfile.TemporaryDirectory() as dest_project_root:
                    # Checkout the repo and copy workflow template to workflows directory
                    cmd = f'git clone --quiet {repo_url}'
                    print(f"Checking out repo {repo_url}: {cmd}")
                    try:
                        subprocess.check_call(cmd, cwd=dest_project_root, shell=True)
                    except subprocess.CalledProcessError as e:
                        raise RuntimeError(e)

                    dest_project_path = os.path.join(dest_project_root, repo)
                    print(f"Copying {workflow} workflow file to {dest_project_path}")
                    copied = copy_workflow_file(workflow_file, dest_project_path)
                    print("Done")
                    if copied:
                        # Push the new workflow file to origin for review
                        # Here may need some customization on the copied workflow
                        workflow_branch = push_to_origin(dest_project_path, workflow, owner, repo)
                        if workflow_branch:
                            create_pull_request(workflow, owner, repo, workflow_branch)


def main():
    args = parse_args()
    repo_url = args.repo_url[0]
    check_workflows(repo_url)


if __name__ == "__main__":
    main()
