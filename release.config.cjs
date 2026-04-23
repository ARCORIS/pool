/**
 * Copyright 2026 The ARCORIS Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/**
 * semantic-release configuration for `arcoris.dev/pool`.
 *
 * This repository is a Go library, not an npm package.
 * Therefore this configuration intentionally avoids npm-specific release steps
 * such as `@semantic-release/npm` and package.json version rewriting.
 *
 * Release model:
 * - `main` publishes stable releases to the default channel
 * - `next` publishes prereleases to the `next` channel
 *
 * Versioning model:
 * - `feat` -> minor
 * - `fix` -> patch
 * - `perf` -> patch
 * - `revert` -> patch
 * - `security` -> patch
 * - `deps` -> patch
 * - any commit containing a breaking-change note -> major
 *
 * Non-release commit types:
 * - `build`
 * - `ci`
 * - `chore`
 * - `docs`
 * - `style`
 * - `refactor`
 * - `test`
 *
 * Important operational note:
 * This file does NOT enforce pull-request approval or branch safety on its own.
 * That must be enforced with:
 * - protected branches / rulesets
 * - required status checks
 * - required reviews
 * - release workflow triggers only on pushes to protected release branches
 *
 * semantic-release itself explicitly warns that any user who can push to a
 * configured release branch can publish a release. Protect those branches.
 *
 * Relevant docs:
 * - semantic-release configuration:
 *   https://semantic-release.gitbook.io/semantic-release/usage/configuration
 * - GitHub protected branches:
 *   https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/
 *   managing-protected-branches/managing-a-branch-protection-rule
 */

/** @type {import('semantic-release').GlobalConfig} */
module.exports = {
    /**
     * Release branches.
     *
     * `main`
     *   Stable releases published to the default distribution channel.
     *
     * `next`
     *   Prereleases published to the `next` channel with prerelease identifiers.
     *
     * This keeps the repository release flow simple:
     * - stable work lands on `main`
     * - preview / pre-release work can flow through `next`
     */
    branches: [
        {
            name: 'main',
            channel: 'latest',
        },
        {
            name: 'next',
            channel: 'next',
            prerelease: 'next',
        },
    ],

    /**
     * Tag format.
     *
     * Keep the default Go-friendly `vX.Y.Z` style.
     * This is the most natural tag shape for a Go library and aligns with normal
     * module-version expectations in the ecosystem.
     */
    tagFormat: 'v${version}',

    /**
     * Plugins run in order.
     *
     * The pipeline is:
     * 1. analyze commits
     * 2. generate release notes
     * 3. update CHANGELOG.md
     * 4. commit the changelog back to git
     * 5. publish the GitHub release
     */
    plugins: [
        [
            '@semantic-release/commit-analyzer',
            {
                /**
                 * Conventional Commits preset.
                 *
                 * The repository expects Conventional Commit style and uses explicit
                 * release rules so that release behavior is visible from this file
                 * instead of being inferred or half-dependent on defaults.
                 */
                preset: 'conventionalcommits',

                /**
                 * Explicit release rules.
                 *
                 * The intent here is conservative:
                 * - user-visible feature additions release a new minor version
                 * - fixes, perf work, security fixes, dependency fixes, and reverts
                 *   release a patch version
                 * - docs/tests/chore/etc. do not release by themselves
                 *
                 * `breaking: true` has priority and upgrades the release to `major`
                 * whenever a breaking-change footer is present.
                 */
                releaseRules: [
                    { breaking: true, release: 'major' },

                    { type: 'feat', release: 'minor' },

                    { type: 'fix', release: 'patch' },
                    { type: 'perf', release: 'patch' },
                    { type: 'revert', release: 'patch' },
                    { type: 'security', release: 'patch' },
                    { type: 'deps', release: 'patch' },

                    { type: 'build', release: false },
                    { type: 'ci', release: false },
                    { type: 'chore', release: false },
                    { type: 'docs', release: false },
                    { type: 'style', release: false },
                    { type: 'refactor', release: false },
                    { type: 'test', release: false },
                ],

                /**
                 * Recognize multiple common breaking-change markers.
                 *
                 * This keeps the configuration explicit and tolerant of the most common
                 * footer spellings used in practice.
                 */
                parserOpts: {
                    noteKeywords: [
                        'BREAKING CHANGE',
                        'BREAKING CHANGES',
                        'BREAKING',
                    ],
                },
            },
        ],

        [
            '@semantic-release/release-notes-generator',
            {
                /**
                 * Use the same Conventional Commits preset for release notes so that
                 * note grouping matches the release decision logic.
                 */
                preset: 'conventionalcommits',

                parserOpts: {
                    noteKeywords: [
                        'BREAKING CHANGE',
                        'BREAKING CHANGES',
                        'BREAKING',
                    ],
                },

                /**
                 * Release-note section policy.
                 *
                 * Only meaningful release-driving categories are shown by default.
                 * Non-release categories are hidden so changelog and GitHub release notes
                 * remain focused and readable.
                 */
                presetConfig: {
                    types: [
                        { type: 'feat', section: 'Features', hidden: false },
                        { type: 'fix', section: 'Bug Fixes', hidden: false },
                        { type: 'perf', section: 'Performance', hidden: false },
                        { type: 'revert', section: 'Reverts', hidden: false },
                        { type: 'security', section: 'Security', hidden: false },
                        { type: 'deps', section: 'Dependencies', hidden: false },

                        { type: 'build', section: 'Build System', hidden: true },
                        { type: 'ci', section: 'Continuous Integration', hidden: true },
                        { type: 'chore', section: 'Chores', hidden: true },
                        { type: 'docs', section: 'Documentation', hidden: true },
                        { type: 'style', section: 'Style', hidden: true },
                        { type: 'refactor', section: 'Refactoring', hidden: true },
                        { type: 'test', section: 'Tests', hidden: true },
                    ],
                },
            },
        ],

        [
            '@semantic-release/changelog',
            {
                /**
                 * Repository changelog file.
                 *
                 * This plugin updates the file during the prepare step so that the git
                 * plugin can commit it back to the release branch.
                 */
                changelogFile: 'CHANGELOG.md',
                changelogTitle: '# Changelog',
            },
        ],

        [
            '@semantic-release/git',
            {
                /**
                 * Only commit files that semantic-release itself updates as part of the
                 * release process.
                 *
                 * For this repository we intentionally keep that narrow:
                 * - CHANGELOG.md
                 *
                 * We do not rewrite Go-specific metadata files or fabricate npm-style
                 * version files for a Go library.
                 */
                assets: [
                    'CHANGELOG.md',
                ],

                /**
                 * Release commit message.
                 *
                 * `[skip ci]` prevents an unnecessary loop when the changelog commit is
                 * pushed back by semantic-release.
                 */
                message:
                    'chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}',
            },
        ],

        [
            '@semantic-release/github',
            {
                /**
                 * Keep repository noise down.
                 *
                 * Releases are still published to GitHub, but the configuration avoids
                 * automatic success/failure comments on issues and pull requests.
                 */
                successComment: false,
                failComment: false,

                /**
                 * Apply a lightweight label to linked pull requests or issues if GitHub
                 * supports the association path in the current workflow.
                 */
                releasedLabels: [
                    'released',
                ],
            },
        ],
    ],
};
