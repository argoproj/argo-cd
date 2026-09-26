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

-- isTargetNotFound reports whether an ancestor entry is an Accepted: False
-- rejection whose reason is TargetNotFound.
--
-- status.ancestors is keyed by (AncestorRef, ControllerName): every conformant
-- Gateway API implementation reconciles every BackendTLSPolicy and writes its
-- own entry under its own controllerName, including a rejection when it cannot
-- resolve the target. A TargetNotFound rejection is therefore ambiguous: it is
-- emitted both by a controller that is not responsible for the target (another
-- implementation that watches the CRD but never owned this target) and by the
-- responsible controller when the policy genuinely points at a resource that
-- does not exist. Without cluster access we cannot tell these apart from a
-- single entry, so these entries are evaluated separately from the other
-- conditions (see below).
function isTargetNotFound(ancestor)
  local accepted = getCondition(ancestor.conditions, "Accepted")
  if accepted == nil then
    return false
  end
  return accepted.status ~= "True" and accepted.reason == "TargetNotFound"
end

-- anotherControllerAccepted reports whether some ancestor written by a
-- controller OTHER than controllerName reports Accepted: True.
--
-- The skip decision for a TargetNotFound rejection is made per controllerName,
-- not globally: status.ancestors is keyed by (AncestorRef, ControllerName), so
-- a TargetNotFound entry is only "another implementation's noise" when a
-- DIFFERENT controller accepted the target. If the SAME controller that
-- emitted the TargetNotFound rejection also reported Accepted: True under its
-- own controllerName, the rejection is a genuine per-controller failure and
-- must degrade the resource; a global "someone accepted" check would wrongly
-- report Healthy in that case.
function anotherControllerAccepted(ancestors, controllerName)
  for _, ancestor in ipairs(ancestors) do
    if ancestor.controllerName ~= controllerName then
      local accepted = getCondition(ancestor.conditions, "Accepted")
      if accepted ~= nil and accepted.status == "True" then
        return true
      end
    end
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

  -- First pass: evaluate every ancestor except the ambiguous TargetNotFound
  -- rejections. A definite Accepted: False (for any other reason) or
  -- ResolvedRefs: False from any controller degrades the resource, and an
  -- Accepted: True marks it healthy. TargetNotFound entries are deferred to the
  -- second pass so that a more specific rejection or an acceptance always wins
  -- over an ambiguous "target not found" signal, regardless of ancestor order.
  for _, ancestor in ipairs(obj.status.ancestors) do
    if ancestor.conditions ~= nil and not isTargetNotFound(ancestor) then
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

  -- Second pass: handle the ambiguous TargetNotFound rejections per controller.
  -- Each rejection is skipped only when a DIFFERENT controller accepted the
  -- target, meaning the rejection belongs to another conformant implementation
  -- that was never going to manage this target. If no other controller
  -- accepted it, the rejection is a real failure and must degrade the resource:
  -- either the policy genuinely points at a resource that does not exist, or
  -- the same controller both accepted and rejected, which is a genuine
  -- per-controller failure rather than another implementation's noise.
  for _, ancestor in ipairs(obj.status.ancestors) do
    if isTargetNotFound(ancestor) and not anotherControllerAccepted(obj.status.ancestors, ancestor.controllerName) then
      local accepted = getCondition(ancestor.conditions, "Accepted")
      hs.status = "Degraded"
      hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. accepted.message
      return hs
    end
  end
end

return hs
