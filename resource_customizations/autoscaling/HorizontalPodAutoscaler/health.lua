local json = require("json")

local progressingStatus = {
  status = "Progressing",
  message = "Waiting to Autoscale",
}

local degradedStates = {
  { type = "AbleToScale", reason = "FailedGetScale" },
  { type = "AbleToScale", reason = "FailedUpdateScale" },
  { type = "ScalingActive", reason = "FailedGetResourceMetric" },
  { type = "ScalingActive", reason = "FailedGetObjectMetric" },
  { type = "ScalingActive", reason = "FailedGetPodsMetric" },
  { type = "ScalingActive", reason = "FailedGetExternalMetric" },
  { type = "ScalingActive", reason = "FailedComputeMetricsReplicas" },
  { type = "ScalingActive", reason = "InvalidSelector" },
}

local function isDegraded(condition)
  for _, degradedState in ipairs(degradedStates) do
    if condition.type == degradedState.type and condition.reason == degradedState.reason then
      return true
    end
  end
  return false
end

local function isHealthy(condition)
  if condition.status == "True" then
    if condition.type == "AbleToScale" or condition.type == "ScalingLimited" then
      return true
    end
  end
  return false
end

local function checkConditions(conditions)
  for _, condition in ipairs(conditions) do
    if isDegraded(condition) then
      return {
        status = "Degraded",
        message = condition.message or "",
      }
    end
  end
  for _, condition in ipairs(conditions) do
    if isHealthy(condition) then
      return {
        status = "Healthy",
        message = condition.message or "",
      }
    end
  end
  return progressingStatus
end

local apiVersion = obj.apiVersion or ""

if apiVersion == "autoscaling/v2" then
  local conditions = {}
  if obj.status ~= nil and obj.status.conditions ~= nil then
    conditions = obj.status.conditions
  end
  return checkConditions(conditions)
end

if apiVersion == "autoscaling/v1" then
  local annotation = nil
  if obj.metadata ~= nil and obj.metadata.annotations ~= nil then
    annotation = obj.metadata.annotations["autoscaling.alpha.kubernetes.io/conditions"]
  end
  if annotation == nil then
    return progressingStatus
  end
  local conditions = json.decode(annotation)
  if conditions == nil or #conditions == 0 then
    return progressingStatus
  end
  return checkConditions(conditions)
end

error("unsupported HPA GVK: " .. apiVersion)
