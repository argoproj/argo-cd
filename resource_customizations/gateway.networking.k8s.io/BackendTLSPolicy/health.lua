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

-- ancestorName returns the ancestorRef name of an ancestor, or an empty string
-- when the ancestorRef or its name is absent. It keeps message building and
-- logging nil-safe when a controller omits parts of the ancestorRef.
function ancestorName(ancestor)
  if ancestor == nil or ancestor.ancestorRef == nil then
    return ""
  end
  return ancestor.ancestorRef.name or ""
end

-- conditionMessage returns a condition's message, or an empty string when it is
-- absent. A conformant controller may record a condition without a message, so
-- every concatenation that includes a message must tolerate a nil value rather
-- than crash the script.
function conditionMessage(condition)
  if condition == nil then
    return ""
  end
  return condition.message or ""
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

-- ancestorRefsEqual reports whether two ancestorRefs identify the same target.
-- It compares the meaningful identifying fields (group, kind, name and
-- namespace) with nil-safety, treating an absent field as an empty string so a
-- controller that omits, for example, the namespace still compares correctly.
function ancestorRefsEqual(a, b)
  if a == nil or b == nil then
    return false
  end
  return (a.group or "") == (b.group or "")
    and (a.kind or "") == (b.kind or "")
    and (a.name or "") == (b.name or "")
    and (a.namespace or "") == (b.namespace or "")
end

-- anotherControllerAcceptedRef reports whether some ancestor written by a
-- controller OTHER than controllerName reports Accepted: True for an ancestorRef
-- that matches targetRef.
--
-- The skip decision for a TargetNotFound rejection is scoped to the SAME target,
-- not made globally: status.ancestors is keyed by (AncestorRef, ControllerName),
-- so a TargetNotFound entry is only "another implementation's noise" when a
-- DIFFERENT controller accepted the SAME ancestorRef. Matching on ancestorRef is
-- what makes this correct: another controller accepting a DIFFERENT target tells
-- us nothing about the rejected target, so it must not suppress the rejection.
-- If the SAME controller that emitted the TargetNotFound rejection also reported
-- Accepted: True, the rejection is a genuine per-controller failure and must
-- degrade the resource; a global "someone accepted" check would wrongly report
-- Healthy in that case.
function anotherControllerAcceptedRef(ancestors, controllerName, targetRef)
  for _, ancestor in ipairs(ancestors) do
    if ancestor.controllerName ~= controllerName and ancestorRefsEqual(ancestor.ancestorRef, targetRef) then
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
              hs.message = "Waiting for Ancestor " .. ancestorName(ancestor) .. " to update BackendTLSPolicy status"
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
            hs.message = "Ancestor " .. ancestorName(ancestor) .. ": " .. conditionMessage(condition)
            return hs
          else
            hs.status = "Healthy"
            hs.message = "BackendTLSPolicy is healthy"
          end
        end

        if condition.type == "ResolvedRefs" then
          if condition.status ~= "True" then
            hs.status = "Degraded"
            hs.message = "Ancestor " .. ancestorName(ancestor) .. ": " .. conditionMessage(condition)
            return hs
          end
        end
      end
    end
  end

  -- Second pass: handle the ambiguous TargetNotFound rejections per controller.
  -- Each rejection is skipped only when a DIFFERENT controller accepted the SAME
  -- ancestorRef, meaning the rejection belongs to another conformant
  -- implementation that was never going to manage this target. If no other
  -- controller accepted that same target, the rejection is a real failure and
  -- must degrade the resource: either the policy genuinely points at a resource
  -- that does not exist, or the same controller both accepted and rejected,
  -- which is a genuine per-controller failure rather than another
  -- implementation's noise.
  for _, ancestor in ipairs(obj.status.ancestors) do
    if isTargetNotFound(ancestor) and not anotherControllerAcceptedRef(obj.status.ancestors, ancestor.controllerName, ancestor.ancestorRef) then
      local accepted = getCondition(ancestor.conditions, "Accepted")
      hs.status = "Degraded"
      hs.message = "Ancestor " .. ancestorName(ancestor) .. ": " .. conditionMessage(accepted)
      return hs
    end
  end
end

return hs
