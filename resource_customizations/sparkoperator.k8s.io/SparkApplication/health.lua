local health_status = {}
-- Can't use standard lib, math.huge equivalent
local infinity = 2^1024-1

local function executor_range_api()
  local min_executor_instances = 0
  local max_executor_instances = infinity
  if obj.spec.dynamicAllocation.maxExecutors then
    max_executor_instances = obj.spec.dynamicAllocation.maxExecutors
  end
  if obj.spec.dynamicAllocation.minExecutors then
    min_executor_instances = obj.spec.dynamicAllocation.minExecutors
  end
  return min_executor_instances, max_executor_instances
end

local function maybe_executor_range_spark_conf()
  local min_executor_instances = 0
  local max_executor_instances = infinity
  if obj.spec.sparkConf["spark.streaming.dynamicAllocation.enabled"] ~= nil and
     obj.spec.sparkConf["spark.streaming.dynamicAllocation.enabled"] == "true" then
    if(obj.spec.sparkConf["spark.streaming.dynamicAllocation.maxExecutors"] ~= nil) then
      max_executor_instances = tonumber(obj.spec.sparkConf["spark.streaming.dynamicAllocation.maxExecutors"])
    end
    if(obj.spec.sparkConf["spark.streaming.dynamicAllocation.minExecutors"] ~= nil) then
      min_executor_instances = tonumber(obj.spec.sparkConf["spark.streaming.dynamicAllocation.minExecutors"])
    end
    return min_executor_instances, max_executor_instances
  elseif obj.spec.sparkConf["spark.dynamicAllocation.enabled"] ~= nil and
     obj.spec.sparkConf["spark.dynamicAllocation.enabled"] == "true" then
    if(obj.spec.sparkConf["spark.dynamicAllocation.maxExecutors"] ~= nil) then
      max_executor_instances = tonumber(obj.spec.sparkConf["spark.dynamicAllocation.maxExecutors"])
    end
    if(obj.spec.sparkConf["spark.dynamicAllocation.minExecutors"] ~= nil) then
      min_executor_instances = tonumber(obj.spec.sparkConf["spark.dynamicAllocation.minExecutors"])
    end
    return min_executor_instances, max_executor_instances
  else
    return nil
  end
end

local function maybe_executor_range()
  if obj.spec["dynamicAllocation"] and obj.spec.dynamicAllocation.enabled then
    return executor_range_api()
  elseif obj.spec["sparkConf"] ~= nil then
    return maybe_executor_range_spark_conf()
  else
    return nil
  end
end

local function dynamic_executors_without_spec_config()
  if obj.spec.dynamicAllocation == nil and obj.spec.executor.instances == nil then
    return true
  else
    return false
  end
end

if obj.status ~= nil then
  if obj.status.applicationState.state ~= nil then
    if obj.status.applicationState.state == "" then
      health_status.status = "Progressing"
      health_status.message = "SparkApplication was added, enqueuing it for submission"
      return health_status
    end
    if obj.status.applicationState.state == "RUNNING" then
      if obj.status.executorState ~= nil then
        count=0
        for i, executorState in pairs(obj.status.executorState) do
          if executorState == "RUNNING" then
            count=count+1
          end
        end
        if obj.spec.executor.instances ~= nil and obj.spec.executor.instances == count then
          health_status.status = "Healthy"
          health_status.message = "SparkApplication is Running"
          return health_status
        elseif maybe_executor_range() then
          local min_executor_instances, max_executor_instances = maybe_executor_range()
          if count >= min_executor_instances and count <= max_executor_instances then
            health_status.status = "Healthy"
            health_status.message = "SparkApplication is Running"
            return health_status
          end
        elseif dynamic_executors_without_spec_config() and count >= 1 then
          health_status.status = "Healthy"
          health_status.message = "SparkApplication is Running"
          return health_status
        end
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
