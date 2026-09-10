-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

-- Health check for the cert-manager trust-manager Bundle CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Bundle reports its state via the "Synced" status condition:
--   Healthy     - Synced is True (the bundle has been synced to all target namespaces).
--   Degraded    - Synced is False (e.g. a missing source ConfigMap/Secret).
--   Progressing - status is not populated yet.
local hs = { status = "Progressing", message = "Waiting for Bundle to be synced" }
if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Synced" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message or "Bundle is synced"
      else
        hs.status = "Degraded"
        hs.message = condition.message or condition.reason or "Bundle is not synced"
      end
      return hs
    end
  end
end
return hs
