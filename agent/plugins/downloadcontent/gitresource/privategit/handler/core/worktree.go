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

// IGitWorktree defines a subset of git.Worktree methods required to clone/checkout a git repository
type IGitWorktree interface {
	Checkout(opts *git.CheckoutOptions) error
}

// Checkout performs the git checkout operation based on the given options
func (gitWorktree *GitWorktree) Checkout(opts *git.CheckoutOptions) error {
	return gitWorktree.Worktree.Checkout(opts)
}

// GitWorktree is a wrapper for git.Worktree and implements IGitWorktree
type GitWorktree struct {
	Worktree *git.Worktree
}

// NewGitWorktree creates a new GitWorktree object
func NewGitWorktree(gitRepository *GitRepository) (gitWorktree IGitWorktree, err error) {
	worktree, err := gitRepository.Repository.Worktree()
	if err != nil {
		return &GitWorktree{
			Worktree: nil,
		}, err
	}

	return &GitWorktree{
		Worktree: worktree,
	}, nil
}
