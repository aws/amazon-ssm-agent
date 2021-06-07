/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

// Package core contains wrapper structs for the git package resources
package core

import (
	"github.com/go-git/go-git/v5"
)

// IGitRepository defines a subset of git.Repository methods required to clone/checkout a git repository
type IGitRepository interface {
	Worktree() (gitWorktree IGitWorktree, err error)
}

// Worktree returns the worktree of the repository
func (repository *GitRepository) Worktree() (gitWorktree IGitWorktree, err error) {
	return NewGitWorktree(repository)
}

// GitRepository is a wrapper for git.Repository and implements IGitRepository
type GitRepository struct {
	Repository *git.Repository
}

// NewGitRepository creates a new GitRepository object
func NewGitRepository(repository *git.Repository) *GitRepository {
	return &GitRepository{
		Repository: repository,
	}
}
