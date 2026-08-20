local health_status = {}
local err = ""

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for Flink operator"
  return health_status
end

if obj.status.error ~= nil and obj.status.error ~= "" then
  err = obj.status.error
end

-- Gate: block Healthy when operator hasn't processed new spec yet
if obj.status.observedGeneration == nil or obj.metadata.generation == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for Flink operator to report generation"
  return health_status
end
if obj.status.observedGeneration < obj.metadata.generation then
  health_status.status = "Progressing"
  health_status.message = "Waiting for operator to reconcile generation " .. obj.metadata.generation
  return health_status
end

-- Reconciliation state (operator-level transitions take priority)
if obj.status.reconciliationStatus ~= nil and obj.status.reconciliationStatus.state ~= nil then
  if obj.status.reconciliationStatus.state == "UPGRADING" then
    health_status.status = "Progressing"
    health_status.message = "Upgrading deployment"
    return health_status
  end
  if obj.status.reconciliationStatus.state == "ROLLING_BACK" then
    health_status.status = "Progressing"
    health_status.message = "Rolling back to last stable spec"
    return health_status
  end
  if obj.status.reconciliationStatus.state == "ROLLED_BACK" then
    health_status.status = "Degraded"
    if err ~= "" then
      health_status.message = "Rolled back: " .. err
    else
      health_status.message = "Deployment rolled back"
    end
    return health_status
  end
end

-- Lifecycle state (primary health signal from operator)
if obj.status.lifecycleState ~= nil then
  if obj.status.lifecycleState == "STABLE" then
    if obj.status.jobStatus ~= nil and obj.status.jobStatus.state == "RESTARTING" then
      health_status.status = "Degraded"
      health_status.message = "Job crash-looping (RESTARTING) despite stable deployment"
      return health_status
    end
    health_status.status = "Healthy"
    health_status.message = "Deployment stable"
    return health_status
  end
  if obj.status.lifecycleState == "FAILED" then
    health_status.status = "Degraded"
    if err ~= "" then
      health_status.message = err
    else
      health_status.message = "Deployment failed"
    end
    return health_status
  end
  if obj.status.lifecycleState == "ROLLING_BACK" then
    health_status.status = "Progressing"
    health_status.message = "Rolling back to last stable spec"
    return health_status
  end
  if obj.status.lifecycleState == "ROLLED_BACK" then
    health_status.status = "Degraded"
    if err ~= "" then
      health_status.message = "Rolled back: " .. err
    else
      health_status.message = "Deployment rolled back"
    end
    return health_status
  end
  if obj.status.lifecycleState == "SUSPENDED" then
    health_status.status = "Suspended"
    health_status.message = "Deployment suspended"
    return health_status
  end
  if obj.status.lifecycleState == "DEPLOYED" then
    health_status.status = "Progressing"
    health_status.message = "Deployed, waiting for stability validation"
    return health_status
  end
  if obj.status.lifecycleState == "UPGRADING" then
    health_status.status = "Progressing"
    health_status.message = "Upgrading deployment"
    return health_status
  end
end

-- Job-level checks (secondary signals, no streaming RUNNING path to Healthy)
if obj.status.jobStatus ~= nil then
  local state = obj.status.jobStatus.state
  if state == "FINISHED" then
    health_status.status = "Healthy"
    health_status.message = "Batch job finished"
    return health_status
  end
  if state == "SUSPENDED" then
    health_status.status = "Suspended"
    health_status.message = "Job suspended"
    return health_status
  end
  if state == "CANCELLING" or state == "CANCELED" then
    health_status.status = "Suspended"
    health_status.message = state
    return health_status
  end
  if state == "RESTARTING" then
    health_status.status = "Progressing"
    health_status.message = "Job restarting"
    return health_status
  end
  if state == "FAILED" then
    health_status.status = "Progressing"
    if err ~= "" then
      health_status.message = "Job failed, operator recovering: " .. err
    else
      health_status.message = "Job failed, operator recovering"
    end
    return health_status
  end
  if state == "FAILING" then
    health_status.status = "Progressing"
    health_status.message = "Job failing, waiting for operator action"
    return health_status
  end
end

-- JobManager-level checks (infrastructure signals)
if obj.status.jobManagerDeploymentStatus == "ERROR" then
  health_status.status = "Progressing"
  if err ~= "" then
    health_status.message = "JobManager error, operator recovering: " .. err
  else
    health_status.message = "JobManager error, operator recovering"
  end
  return health_status
end
if obj.status.jobManagerDeploymentStatus == "DEPLOYING" then
  health_status.status = "Progressing"
  health_status.message = "JobManager deploying"
  return health_status
end
if obj.status.jobManagerDeploymentStatus == "MISSING" then
  health_status.status = "Progressing"
  health_status.message = "JobManager missing, waiting for recovery"
  return health_status
end

health_status.status = "Progressing"
health_status.message = "Waiting for Flink operator"
return health_status

health_status.status = "Progressing"
health_status.message = "Waiting for Flink operator"
return health_status
