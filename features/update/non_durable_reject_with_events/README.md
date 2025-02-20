# Update Rejections Do Not Take Up Space In History

When updates are registered with a validation handler and that validation
handler rejects an update, neither the invocation of the update nor the
rejection appear as part of the workflow's history.

# Detailed spec

Update requests don't hit history until they've passed validation on a worker.
Update rejections never hit history. If a workflow task carries _only_ update
requests and all of those requests are rejected, the events related to that
workflow task (i.e. Scheduled/Started/Completed) do not land in history.
