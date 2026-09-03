local hs = {
  status = "Progressing",
  message = "Waiting for CloudNativePG to reconcile Database"
}

if obj.status == nil then
  return hs
end

-- CloudNativePG only advances status.observedGeneration on a successful
-- reconcile (SetAsReady); SetAsFailed and SetAsUnknown leave it untouched. So
-- observedGeneration == metadata.generation means the current spec generation
-- has been reconciled at least once. See database_funcs.go:
-- https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.0/api/v1/database_funcs.go
local generationObserved =
  obj.metadata == nil or obj.metadata.generation == nil or
  obj.status.observedGeneration == obj.metadata.generation

if obj.status.applied == true and generationObserved then
  hs.status = "Healthy"
  hs.message = obj.status.message or ""
  return hs
end

-- A failed reconcile of the generation CloudNativePG has already observed is a
-- genuine regression of a Database that was previously applied -> Degraded.
if obj.status.applied == false and generationObserved then
  hs.status = "Degraded"
  hs.message = obj.status.message or "CloudNativePG failed to apply the Database"
  return hs
end

-- Otherwise the current generation has not been applied yet: a first reconcile,
-- a pending spec change, or a failure that happened before the current
-- generation was ever observed. Report Progressing (NOT Degraded) so the sync
-- wave waits. On first adoption the Database's owner role may not exist yet, so
-- CloudNativePG's first reconcile fails (applied=false) and only succeeds on a
-- retry; Argo CD turns a Degraded non-hook resource into a failed sync
-- operation, so degrading this self-healing state would fail the very sync this
-- health check is meant to gate.
if obj.status.message ~= nil and obj.status.message ~= "" then
  hs.message = obj.status.message
end
return hs
