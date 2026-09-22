local hs = {}

-- getCondition returns the condition of the given type from an ancestor's
-- conditions, or nil when it is absent.
function getCondition(conditions, conditionType)
  if conditions == nil then
    return nil
  end
  for _, condition in ipairs(conditions) do
    if condition.type == conditionType then
      return condition
    end
  end
  return nil
end

-- isNotResponsible reports whether an ancestor entry belongs to a controller
-- that is not responsible for the target of this BackendTLSPolicy.
--
-- status.ancestors is a map keyed by (AncestorRef, ControllerName): every
-- conformant Gateway API implementation in the cluster reconciles every
-- BackendTLSPolicy and writes its own entry under its own controllerName,
-- including a rejection when it cannot resolve the target. Such an entry
-- reports Accepted: False with reason TargetNotFound and must not, on its own,
-- drive the resource Degraded, because the controller actually responsible for
-- the target may have accepted the policy under a different entry.
function isNotResponsible(ancestor)
  local accepted = getCondition(ancestor.conditions, "Accepted")
  if accepted == nil then
    return false
  end
  if accepted.status ~= "True" and accepted.reason == "TargetNotFound" then
    return true
  end
  return false
end

hs.status = "Progressing"
hs.message = "Waiting for BackendTLSPolicy status"

if obj.status ~= nil and obj.status.ancestors ~= nil then
  if obj.metadata.generation ~= nil then
    for _, ancestor in ipairs(obj.status.ancestors) do
      if ancestor.conditions ~= nil then
        for _, condition in ipairs(ancestor.conditions) do
          if condition.observedGeneration ~= nil then
            if condition.observedGeneration ~= obj.metadata.generation then
              hs.message = "Waiting for Ancestor " .. (ancestor.ancestorRef.name or "") .. " to update BackendTLSPolicy status"
              return hs
            end
          end
        end
      end
    end
  end

  for _, ancestor in ipairs(obj.status.ancestors) do
    -- Skip entries written by controllers that are not responsible for the
    -- target. These belong to other conformant implementations that watch the
    -- CRD but were never going to manage this target, so their rejection must
    -- not affect the reported health.
    if ancestor.conditions ~= nil and not isNotResponsible(ancestor) then
      for _, condition in ipairs(ancestor.conditions) do
        if condition.type == "Accepted" then
          if condition.status ~= "True" then
            hs.status = "Degraded"
            hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. condition.message
            return hs
          else
            hs.status = "Healthy"
            hs.message = "BackendTLSPolicy is healthy"
          end
        end

        if condition.type == "ResolvedRefs" then
          if condition.status ~= "True" then
            hs.status = "Degraded"
            hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. condition.message
            return hs
          end
        end
      end
    end
  end
end

return hs
