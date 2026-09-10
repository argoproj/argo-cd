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

local health_status = {}

if obj.status ~= nil and obj.status.reconciliationStatus ~= nil then
  if obj.status.reconciliationStatus.success or obj.status.reconciliationStatus.state == "DEPLOYED" then
    health_status.status = "Healthy"
    return health_status
  end 

  if obj.status.jobManagerDeploymentStatus == "DEPLOYED_NOT_READY" or obj.status.jobManagerDeploymentStatus == "DEPLOYING" then
    health_status.status = "Progressing"
    health_status.message = "Waiting for deploying"
    return health_status
  end

  if obj.status.jobManagerDeploymentStatus == "ERROR" then
    health_status.status = "Degraded"
    health_status.message = obj.status.reconciliationStatus.error
    return health_status
  end 
end

-- A FlinkDeployment submitted with `job.state: suspended` is never deployed: the
-- operator returns before its first deployment, so no `lastReconciledSpec` is ever
-- recorded, `reconciliationStatus` keeps its default `UPGRADING` state and
-- `jobManagerDeploymentStatus` stays `MISSING`. The deployment already is in its
-- desired state, so waiting for the operator here would keep it Progressing forever.
-- A job suspended after it had been deployed is not matched here: it records a
-- `lastReconciledSpec` and is still converging, so it stays Progressing.
if obj.spec ~= nil and obj.spec.job ~= nil and obj.spec.job.state == "suspended"
    and (obj.status == nil or obj.status.reconciliationStatus == nil
      or obj.status.reconciliationStatus.lastReconciledSpec == nil) then
  health_status.status = "Healthy"
  health_status.message = "Job is suspended"
  return health_status
end

health_status.status = "Progressing"
health_status.message = "Waiting for Flink operator"
return health_status
