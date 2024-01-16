health_status = {}
health_status.status = "Progressing"
health_status.message = "Waiting for status update."
if obj.status ~= nil and obj.status.conditions ~= nil then
  status_true = 0
  status_false = 0
  status_unknown = 0
  health_status.message = ""
  for i, condition in pairs(obj.status.conditions) do
    if condition.status == "True" and (condition.type == "IngressReady" or condition.type == "PredictorConfigurationReady" or condition.type == "PredictorReady" or condition.type == "PredictorRouteReady" or condition.type == "Ready") then
      status_true = status_true + 1
    elseif condition.status == "False" or condition.status == "Unknown" then
      msg = condition.type .. " is " .. condition.status
      if condition.reason ~= nil and condition.reason ~= "" then
        msg = msg .. ", since " .. condition.reason .. "."
      end
      if condition.message ~= nil and condition.message ~= "" then
        msg = msg .. " " .. condition.message
      end
      health_status.message = health_status.message .. msg .. "\n"
      if condition.status == "False" then
        status_false = status_false + 1
      else
        status_unknown = status_unknown + 1
      end
    end
  end
  if status_true == 5 and status_false == 0 and status_unknown == 0 then
    health_status.message = "Inference Service is healthy."
    health_status.status = "Healthy"
    return health_status
  elseif status_false > 0 then
    health_status.status = "Degraded"
    return health_status
  else
    health_status.status = "Progressing"
    return health_status
  end
end
return health_status