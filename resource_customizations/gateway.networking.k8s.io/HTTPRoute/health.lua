local hs = {}

function checkConditions(conditions, conditionType)
  for _, condition in ipairs(conditions) do
    if condition.type == conditionType and condition.status == "False" then
      return false, condition.message or ("Failed condition: " .. conditionType)
    end
  end
  return true
end

-- isParentGenerationObserved checks if a parent's conditions match the current resource generation
-- For HTTPRoute, observedGeneration is stored in each condition within a parent
function isParentGenerationObserved(obj, parent)
  if obj.metadata.generation == nil then
    -- If no generation is set, accept all conditions
    return true
  end

  if parent.conditions == nil or #parent.conditions == 0 then
    return false
  end

  -- Check if all conditions have observedGeneration matching current generation
  for _, condition in ipairs(parent.conditions) do
    if condition.observedGeneration ~= nil then
      if condition.observedGeneration ~= obj.metadata.generation then
        return false
      end
    end
  end

  return true
end

if obj.status ~= nil then
  if obj.status.parents ~= nil then
    for _, parent in ipairs(obj.status.parents) do
      if parent.conditions ~= nil then
        -- Skip this parent if it's not from the current generation
        if not isParentGenerationObserved(obj, parent) then
          goto continue
        end

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

        ::continue::
      end
    end

    if #obj.status.parents > 0 then
      for _, parent in ipairs(obj.status.parents) do
        if parent.conditions ~= nil and #parent.conditions > 0 then
          -- Only mark as healthy if we found a parent from the current generation
          if isParentGenerationObserved(obj, parent) then
            hs.status = "Healthy"
            hs.message = "HTTPRoute is healthy"
            return hs
          end
        end
      end
    end
  end
end

-- Some Gateway API implementations never populate .status.parents even though the
-- route is functional. A health check cannot time out, so reporting "Progressing"
-- here would leave the Application progressing forever (see issue #28352). When no
-- controller has reported any parent status, assume the route is healthy, matching
-- how Argo CD treats resources without a health check.
if obj.status == nil or obj.status.parents == nil or #obj.status.parents == 0 then
  hs.status = "Healthy"
  hs.message = "No status reported by the Gateway controller; assuming HTTPRoute is healthy"
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for HTTPRoute status"
return hs
