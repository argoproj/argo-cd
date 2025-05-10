local hs = {}

function checkConditions(conditions, conditionType)
  for _, condition in ipairs(conditions) do
    if condition.type == conditionType and condition.status == "False" then
      return false, condition.message or ("Failed condition: " .. conditionType)
    end
  end
  return true
end

if obj.status ~= nil then
  if obj.status.conditions ~= nil then
    local resolvedRefsFalse, resolvedRefsMsg = checkConditions(obj.status.conditions, "ResolvedRefs")
    local acceptedFalse, acceptedMsg = checkConditions(obj.status.conditions, "Accepted")

    if not resolvedRefsFalse then
      hs.status = "Degraded"
      hs.message = resolvedRefsMsg
      return hs
    end

    if not acceptedFalse then
      hs.status = "Degraded"
      hs.message = acceptedMsg
      return hs
    end

    local isProgressing = false
    local progressingMsg = ""

    for _, condition in ipairs(obj.status.conditions) do
      if condition.type == "Programmed" and condition.status ~= "True" then
        isProgressing = true
        progressingMsg = condition.message or "Gateway is still being programmed"
        break
      end
    end

    if isProgressing then
      hs.status = "Progressing"
      hs.message = progressingMsg
      return hs
    end
  end

  if obj.status.listeners ~= nil then
    for _, listener in ipairs(obj.status.listeners) do
      if listener.conditions ~= nil then
        local resolvedRefsFalse, resolvedRefsMsg = checkConditions(listener.conditions, "ResolvedRefs")
        local acceptedFalse, acceptedMsg = checkConditions(listener.conditions, "Accepted")

        if not resolvedRefsFalse then
          hs.status = "Degraded"
          hs.message = "Listener: " .. resolvedRefsMsg
          return hs
        end

        if not acceptedFalse then
          hs.status = "Degraded"
          hs.message = "Listener: " .. acceptedMsg
          return hs
        end

        local isProgressing = false
        local progressingMsg = ""

        for _, condition in ipairs(listener.conditions) do
          if condition.type == "Programmed" and condition.status ~= "True" then
            isProgressing = true
            progressingMsg = condition.message or "Listener is still being programmed"
            break
          end
        end

        if isProgressing then
          hs.status = "Progressing"
          hs.message = "Listener: " .. progressingMsg
          return hs
        end
      end
    end
  end

  if obj.status.conditions ~= nil or (obj.status.listeners ~= nil and #obj.status.listeners > 0) then
    hs.status = "Healthy"
    hs.message = "Gateway is healthy"
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for Gateway status"
return hs
