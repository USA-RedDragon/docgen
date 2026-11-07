# Site outline

The pages of the docs site, in reading order, by section. The site navigation comes from this list.

- [Overview](../README.md)
- Get started
  - [Install](start/01-install.md)
  - [Your first page](start/02-first-page.md)
  - [Check the docs in CI](start/03-check-in-ci.md)
- Guides
  - [Show a constant](guides/const.md)
  - [Show code](guides/code.md)
  - [Show output](guides/output.md)
  - [Show command usage](guides/flags.md)
  - [Document a config struct](guides/config-tables.md)
  - [Document a configulator config](guides/configulator.md)
  - [Document errors](guides/errors.md)
  - [Document a state machine](guides/state-machines.md)
  - [Show a doc comment](guides/doc-comments.md)
  - [Show a go.mod value](guides/gomod.md)
  - [Use variables](guides/variables.md)
  - [Write a code block with variables](guides/text.md)
  - [Run docgen on docgen](guides/sample.md)
  - [Use a module in a subdirectory](guides/module-in-subdirectory.md)
- Concepts
  - [How docgen works](concepts/how-it-works.md)
- Reference
  - [Markers](reference/markers.md)
  - [Command line](reference/command.md)
  - [Sample page](reference/sample-page.md)
- Project
  - [Versioning and compatibility](project/versioning.md)
  - [Tooling and checks](project/tooling.md)

Each item is a link to a page, or, for a page that is planned and not written, its title and its path in code. A comment `<!-- old: <path> -->` after a link gives the path of the page before it moved; the site uses that file until the new one exists. A link to a file that is not a page, or to another site, shows in the navigation and opens that file or site.
