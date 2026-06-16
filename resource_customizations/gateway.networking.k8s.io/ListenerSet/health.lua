local hs = { status = "Progressing", message = "Waiting for ListenerSet status"}

function checkConditions(conditions, conditionType)
  for _, condition in ipairs(conditions) do
    if condition.type == conditionType and condition.status == "False" then
      return false, condition.message or ("Failed condition: " .. conditionType)
    end
  end
  return true
end

if obj.status ~= nil then
  if obj.metadata.generation ~= nil then
    if obj.status.conditions ~= nil then
      for _, condition in ipairs(obj.status.conditions) do
        if condition.observedGeneration ~= nil then
          if condition.observedGeneration ~= obj.metadata.generation then
              return hs
          end
        end
      end
    end
  end

  local acceptedFalse, acceptedMsg = checkConditions(obj.status.conditions, "Accepted")
  if not acceptedFalse then
    hs.status = "Degraded"
    hs.message = acceptedMsg
    return hs
  end

  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Programmed" and condition.status ~= "True" then
      hs.status = "Progressing"
      hs.message =  condition.message or "ListenerSet is still being programmed"
      return hs
    end
  end

  if obj.status.listeners ~= nil and #obj.status.listeners > 0 then
    for _, listener in ipairs(obj.status.listeners) do
      if listener.conditions ~= nil then
        local resolvedRefsFalse, resolvedRefsMsg = checkConditions(listener.conditions, "ResolvedRefs")
        if not resolvedRefsFalse then
          hs.status = "Degraded"
          hs.message = "Listener: " .. resolvedRefsMsg
          return hs
        end

        local acceptedFalse, acceptedMsg = checkConditions(listener.conditions, "Accepted")
        if not acceptedFalse then
          hs.status = "Degraded"
          hs.message = "Listener: " .. acceptedMsg
          return hs
        end
        for _, condition in ipairs(listener.conditions) do
          if condition.type == "Programmed" and condition.status ~= "True" then
            hs.status = "Progressing"
            hs.message = "Listener: " .. condition.message or "Listener is still being programmed"
            return hs
          end
        end
      end
    end

    hs.status = "Healthy"
    hs.message = "ListenerSet is healthy"
    return hs
  end
end

return hs
