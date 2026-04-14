package task

// Validate runs the parse + schema + version checks on the configured
// Ritefile without executing any tasks, evaluating any `sh:` dynamic vars,
// or reading dotenv files. It's the machinery behind `rite --validate` and
// is intended for editor extensions, pre-commit hooks, and CI lint stages.
//
// The check chain is a strict subset of Setup():
//  1. Discover the root Ritefile (respecting Entrypoint / Dir).
//  2. Parse YAML, merge `includes:` (cycle detection happens here).
//  3. Version check against the supported schema range.
//
// On success, e.Ritefile is populated — callers can layer additional
// semantic checks on top. On failure, returns the same typed errors
// Setup() would, so exit codes route through the existing dispatcher
// (CodeRitefileDecode=102, CodeRitefileInvalid=104, CodeRitefileCycle=105,
// CodeRitefileVersionCheckError=103, CodeRitefileNotFound=100).
func (e *Executor) Validate() error {
	e.setupLogger()
	node, err := e.getRootNode()
	if err != nil {
		return err
	}
	if err := e.readTaskfile(node); err != nil {
		return err
	}
	return e.doVersionChecks()
}
