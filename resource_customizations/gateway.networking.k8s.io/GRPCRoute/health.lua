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
  if obj.status.parents ~= nil then
    for _, parent in ipairs(obj.status.parents) do
      if parent.conditions ~= nil then
        local resolvedRefsFalse, resolvedRefsMsg = checkConditions(parent.conditions, "ResolvedRefs")
        local acceptedFalse, acceptedMsg = checkConditions(parent.conditions, "Accepted")

        if not resolvedRefsFalse then
          hs.status = "Degraded"
          hs.message = "Parent " .. (parent.parentRef.name or "") .. ": " .. resolvedRefsMsg
          return hs
        end

        if not acceptedFalse then
          hs.status = "Degraded"
          hs.message = "Parent " .. (parent.parentRef.name or "") .. ": " .. acceptedMsg
          return hs
        end

        local isProgressing = false
        local progressingMsg = ""

        for _, condition in ipairs(parent.conditions) do
          if condition.type == "Programmed" and condition.status ~= "True" then
            isProgressing = true
            progressingMsg = condition.message or "Route is still being programmed"
            break
          end
        end

        if isProgressing then
          hs.status = "Progressing"
          hs.message = "Parent " .. (parent.parentRef.name or "") .. ": " .. progressingMsg
          return hs
        end
      end
    end

    if #obj.status.parents > 0 then
      for _, parent in ipairs(obj.status.parents) do
        if parent.conditions ~= nil and #parent.conditions > 0 then
          hs.status = "Healthy"
          hs.message = "GRPCRoute is healthy"
          return hs
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for GRPCRoute status"
return hs
