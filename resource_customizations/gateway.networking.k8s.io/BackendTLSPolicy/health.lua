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

-- anyAncestorAccepted reports whether any ancestor entry reports Accepted: True,
-- i.e. some controller in the cluster actually accepted and owns the target of
-- this BackendTLSPolicy.
function anyAncestorAccepted(ancestors)
  for _, ancestor in ipairs(ancestors) do
    local accepted = getCondition(ancestor.conditions, "Accepted")
    if accepted ~= nil and accepted.status == "True" then
      return true
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

  -- Second pass: handle the ambiguous TargetNotFound rejections. If some
  -- controller already accepted the target, these belong to other conformant
  -- implementations that were never going to manage this target, so they are
  -- skipped and must not affect the reported health. If no controller accepted
  -- the target, the policy genuinely points at a resource that does not exist,
  -- so the rejection is a real failure and must degrade the resource.
  if not anyAncestorAccepted(obj.status.ancestors) then
    for _, ancestor in ipairs(obj.status.ancestors) do
      if isTargetNotFound(ancestor) then
        local accepted = getCondition(ancestor.conditions, "Accepted")
        hs.status = "Degraded"
        hs.message = "Ancestor " .. (ancestor.ancestorRef.name or "") .. ": " .. accepted.message
        return hs
      end
    end
  end
end

return hs
