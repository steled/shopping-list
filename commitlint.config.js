/**
 * Commitlint configuration
 *
 * Enforces conventional commits format:
 * type(scope): subject
 *
 * Examples:
 *   feat(api): add tag filter to items endpoint
 *   fix(ui): correct dark mode transition on reload
 *   docs(readme): add helm install instructions
 *   chore(deps): bump alpine base image to 3.22
 */

module.exports = {
  extends: ['@commitlint/config-conventional'],
  ignores: [
    (message) => /^(feat|fix|chore)\(deps\):/.test(message),
  ],
  rules: {
    'type-enum': [
      2,
      'always',
      [
        'feat',
        'fix',
        'docs',
        'style',
        'refactor',
        'perf',
        'test',
        'chore',
        'ci',
        'build',
        'revert',
      ],
    ],
    'subject-case': [2, 'always', 'lower-case'],
    'subject-max-length': [2, 'always', 100],
    'subject-empty': [2, 'never'],
    'body-leading-blank': [2, 'always'],
    'footer-leading-blank': [2, 'always'],
    'scope-empty': [0, 'never'],
  },
};
