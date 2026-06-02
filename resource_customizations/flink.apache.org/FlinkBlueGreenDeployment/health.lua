local health_status = {}

if obj.status == nil then
  health_status.status = "Progressing"
  health_status.message = "Waiting for Flink operator"
  return health_status
end

local err = ""
if obj.status.error ~= nil and obj.status.error ~= "" then
  err = obj.status.error
end

-- Reconciliation state (operator-level transitions take priority)
-- Note: FlinkBlueGreenDeployment has no observedGeneration field;
-- the blueGreenState machine itself provides rollback safety.
if obj.status.reconciliationStatus ~= nil and obj.status.reconciliationStatus.state ~= nil then
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

-- BlueGreen state machine
if obj.status.blueGreenState ~= nil then
  if obj.status.blueGreenState == "ACTIVE_BLUE" or obj.status.blueGreenState == "ACTIVE_GREEN" then
    -- Active deployment exists, but check if the job is crash-looping
    if obj.status.jobStatus ~= nil and obj.status.jobStatus.state == "RESTARTING" then
      health_status.status = "Degraded"
      health_status.message = "Job crash-looping (RESTARTING) in " .. obj.status.blueGreenState
      return health_status
    end
    health_status.status = "Healthy"
    if err ~= "" then
      health_status.message = obj.status.blueGreenState .. " (warning: " .. err .. ")"
    else
      health_status.message = obj.status.blueGreenState
    end
    return health_status
  end
  if obj.status.blueGreenState == "INITIALIZING_BLUE" then
    health_status.status = "Progressing"
    health_status.message = "Initializing blue deployment"
    return health_status
  end
  if obj.status.blueGreenState == "TRANSITIONING_TO_BLUE" then
    health_status.status = "Progressing"
    health_status.message = "Rolling back to blue deployment"
    return health_status
  end
  if obj.status.blueGreenState == "TRANSITIONING_TO_GREEN" then
    health_status.status = "Progressing"
    health_status.message = "Transitioning to green deployment"
    return health_status
  end
  if obj.status.blueGreenState == "SAVEPOINTING_BLUE" or obj.status.blueGreenState == "SAVEPOINTING_GREEN" then
    health_status.status = "Progressing"
    health_status.message = "Taking savepoint: " .. obj.status.blueGreenState
    return health_status
  end
end

-- Job-level fallback
if obj.status.jobStatus ~= nil and obj.status.jobStatus.state == "FAILED" then
  health_status.status = "Progressing"
  if err ~= "" then
    health_status.message = "Job failed, operator recovering: " .. err
  else
    health_status.message = "Job failed, operator recovering"
  end
  return health_status
end

health_status.status = "Progressing"
if err ~= "" then
  health_status.message = "Operator processing: " .. err
else
  health_status.message = "Waiting for Flink operator"
end
return health_status
