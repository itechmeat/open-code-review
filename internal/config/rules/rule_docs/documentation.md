Documentation files are reviewed only when the run opts in (`--include-docs` or an include pattern). Review what the text claims, not how it is written.

#### Claims against the code
- **Changelog and release notes**: every entry describes a change that this diff (or the code it points to) actually makes; flag entries for behavior that is absent, different, or only partly implemented, and user-visible changes in the diff that the changelog omits
- **Commands, flags, options and config keys**: names, defaults, accepted values and exit codes match the code that defines them; use code_search and file_read to confirm before reporting
- **Paths, file names, APIs and identifiers**: referenced files, functions, types, environment variables and endpoints exist under that name
- **Examples and snippets**: commands and code samples would run as written against the current code (arguments, output shape, required setup)
- **Security and safety statements**: guarantees such as "never", "always", "validated" or "sandboxed" are backed by the code paths they describe

#### Consistency
- Statements that contradict another place in the same change (another doc, a translation, a code comment or help text)
- Version numbers, counts and limits that disagree with the code or the manifest files in the diff

#### Out of scope
- Do not report wording, tone, grammar, formatting or Markdown style unless the text is wrong or misleading
- Do not report missing documentation for code that the diff does not change
