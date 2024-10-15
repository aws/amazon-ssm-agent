#!/usr/bin/env bash
changed_golang_packages=$(git status --porcelain | awk '{print $2}' | grep '\.go$' | xargs dirname | sort | uniq)

if  [[ -x "$(command -v golangci-lint)" ]]; then
  echo "golangci-lint executable found."
  echo "Running golangci-lint on changed files"
  for package_dir in $changed_golang_packages
  do
    $(command -v golangci-lint) run "$package_dir/..."
  done
else
  bold=$(tput bold)
  normal=$(tput sgr0)
  echo "${bold}golangci-lint executable not found. See https://golangci-lint.run/usage/install/ for installation instructions.${normal}"
  exit 1
fi
