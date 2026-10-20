# Updating Factory and team tools

Factory and team tools are independent checkouts. Update each deliberately using your normal repository workflow, then run `npm ci --omit=dev` in the framework when dependencies change. Existing sessions retain prepared configuration; restart them to use updated code and resources.

Keep project overrides in the application's `factory.json`, extending the team's external configuration. Framework updates do not overwrite that file or the tools repository. Changes in the referenced team configuration affect future launches, so review them before updating. Pin checkout revisions when reproducibility matters.

No framework or tools copy is required in an application. The optional `sync_toolkit.ts` copier is only for maintaining a separate copied framework checkout; it is not part of the normal setup.
