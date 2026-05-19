local hs = {}

if obj ~= nil and obj.status ~= nil then
  local deployments = obj.status.deployments
  if deployments ~= nil then
    local has_ready = deployments.ready ~= nil
    local has_starting = deployments.starting ~= nil
    local has_stopped = deployments.stopped ~= nil

    if has_ready and (not has_starting) and (not has_stopped) then
      hs.status = "Healthy"
      hs.message = "3scale APIManager is available"
      return hs
    elseif (has_ready and (has_starting or has_stopped)) or (not has_ready) then
      hs.status = "Progressing"
      hs.message = "Waiting for 3scale APIManager status..."
    end
  end

  -- Fallback to condition-based evaluation
  if obj.status.conditions ~= nil then
    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Available" then
        if condition.status == "True" then
          hs.status = "Healthy"
          hs.message = "3scale APIManager is available"
          return hs
        elseif condition.reason ~= nil and condition.reason ~= "" then
          local msg = "3scale APIManager is degraded: " .. condition.reason
          hs.status = "Degraded"
          hs.message = msg
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for 3scale APIManager status..."
return hs
