#!/bin/bash

# Script to collect GitHub handle and branch name from user

echo "Please provide the following information:"
echo

# Prompt for GitHub handle
read -r -p "Enter your GitHub handle: " github_handle

# Validate that GitHub handle is not empty
while [[ -z "$github_handle" ]]; do
    echo "GitHub handle cannot be empty."
    read -r -p "Enter your GitHub handle: " github_handle
done

# Prompt for branch name
read -r -p "Enter the branch name: " branch_name

# Validate that branch name is not empty
while [[ -z "$branch_name" ]]; do
    echo "Branch name cannot be empty."
    read -r -p "Enter the branch name: " branch_name
done

echo "Merging migrated gitops-engine branch $branch_name from $github_handle/gitops-engine.git"

git remote add ge-migrated git@github.com:"$github_handle"/gitops-engine.git
git fetch ge-migrated

if git merge --no-edit ge-migrated/"$branch_name" --allow-unrelated-histories ; then
    sh ./hack/migrate-gitops-engine/replace-vendor.sh
    sh ./hack/migrate-gitops-engine/update-dockerfile.sh
    echo "Merging of gitops-engine branch $branch_name from $github_handle/gitops-engine.git succeeded"
else
    echo "Merging of gitops-engine branch $branch_name from $github_handle/gitops-engine.git failed"
    exit 1
fi


