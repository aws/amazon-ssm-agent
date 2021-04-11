#!/usr/bin/env bash

if  [[ -x "$(command -v golangci-lint)" ]]; then
  echo "golangci-lint executable found."
  echo "Run golangci-lint"
  $(command -v golangci-lint) run ./agent/... ./core/... ./common/...
else
  bold=$(tput bold)
  normal=$(tput sgr0)
  echo "${bold}golangci-lint executable not found. See https://golangci-lint.run/usage/install/ for installation instructions.${normal}"
  exit 1
fi
