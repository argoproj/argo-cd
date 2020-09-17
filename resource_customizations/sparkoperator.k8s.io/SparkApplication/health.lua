health_status = {}
if obj.status ~= nil then
  if obj.status.applicationState.state ~= nil then
    if obj.status.applicationState.state == "" then
      health_status.status = "Progressing"
      health_status.message = "SparkApplication was added, enqueuing it for submission"
      return health_k9sstatus
    end
    count=0
    executor_instances = obj.spec.executor.instances
    for i, executorState in pairs(obj.status.executorState) do
      if executorState == "RUNNING" then
          count=count+1
        end
    end
    if executor_instances == count then
      if obj.status.applicationState.state == "RUNNING" then
        health_status.status = "Healthy"
        health_status.message = "SparkApplication is in RunningState"
        return health_status
      end
    end
    if obj.status.applicationState.state == "SUBMITTED" then
      health_status.status = "Progressing"
      health_status.message = "SparkApplication was submitted successfully"
      return health_status
    end
    if obj.status.applicationState.state == "COMPLETED" then
      health_status.status = "Healthy"
      health_status.message = "SparkApplication was Completed"
      return health_status
    end
    if obj.status.applicationState.state == "FAILED" then
      health_status.status = "Degraded"
      health_status.message = obj.status.applicationState.errorMessage
      return health_status
    end
    if obj.status.applicationState.state == "SUBMISSION_FAILED" then
      health_status.status = "Degraded"
      health_status.message = obj.status.applicationState.errorMessage
      return health_status
    end
    if obj.status.applicationState.state == "PENDING_RERUN" then
      health_status.status = "Progressing"
      health_status.message = "SparkApplication is Pending Rerun"
      return health_status
    end
    if obj.status.applicationState.state == "INVALIDATING" then
      health_status.status = "Missing"
      health_status.message = "SparkApplication is in InvalidatingState"
      return health_status
    end
    if obj.status.applicationState.state == "SUCCEEDING" then
      health_status.status = "Progressing"
      health_status.message = [[The driver pod has been completed successfully. The executor pods terminate and are cleaned up.
                                Under this circumstances, we assume the executor pod are completed.]]
      return health_status
    end
    if obj.status.applicationState.state == "FAILING" then
      health_status.status = "Degraded"
      health_status.message = obj.status.applicationState.errorMessage
      return health_status
    end
    if obj.status.applicationState.state == "UNKNOWN" then
      health_status.status = "Progressing"
      health_status.message = "SparkApplication is in UnknownState because either driver pod or one or all executor pods in unknown state  "
      return health_status
    end
  end
end
health_status.status = "Progressing"
health_status.message = "Waiting for Executor pods"
return health_status