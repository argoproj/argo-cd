local hs = { status = "Progressing", message = "Waiting for ListenerSet status"}

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

  local isProgressing = false
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Accepted" and condition.status == "False" then
      hs.status = "Degraded"
      hs.message = condition.message or ("Failed condition: " .. conditionType)
      return hs
    end

    if condition.type == "Programmed" and condition.status ~= "True" then
      isProgressing = true
      hs.status = "Progressing"
      hs.message =  condition.message or "ListenerSet is still being programmed"
    end
  end

  if isProgressing then
    return hs
  end

  if obj.status.listeners ~= nil and #obj.status.listeners > 0 then
    for _, listener in ipairs(obj.status.listeners) do
      if listener.conditions ~= nil then
        local isProgressing = false
        for _, condition in ipairs(listener.conditions) do
            if condition.type == "ResolvedRefs" and condition.status == "False" then
              hs.status = "Degraded"
              hs.message = "Listener: " .. (condition.message or ("Failed condition: " .. conditionType))
              return hs
            end

            if condition.type == "Conflicted" and condition.status == "True" then
              hs.status = "Degraded"
              hs.message = "Listener: " .. (condition.message or "Listener is conflicted")
              return hs
            end

            if condition.type == "Accepted" and condition.status == "False" then
              hs.status = "Degraded"
              hs.message = "Listener: " .. (condition.message or ("Failed condition: " .. conditionType))
              return hs
            end

            if condition.type == "Programmed" and condition.status ~= "True" then
                isProgressing = true
                hs.status = "Progressing"
                hs.message = "Listener: " .. (condition.message or "Listener is still being programmed")
            end
        end

        if isProgressing then
          return hs
        end
      end
    end

    hs.status = "Healthy"
    hs.message = "ListenerSet is healthy"
    return hs
  end
end

return hs
