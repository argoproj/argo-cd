-- TLSRoute health check for Argo CD
--
-- TLSRoute is a Gateway API resource for TLS-passthrough and SNI-based routing.
-- It is GA and part of the Standard Channel since Gateway API v1.5.0.
-- Its readiness is expressed through .status.parents[].conditions, where each
-- parent entry represents a Gateway that the route is attached to.
--
-- Gateway API communicates controller state via Conditions on each parent entry.
-- Ref: https://gateway-api.sigs.k8s.io/api-types/tlsroute/
--
-- Commonly used route condition types:
--   Accepted     – the route is accepted by the parent Gateway
--   ResolvedRefs – all backend references are valid
--   Programmed   – the route has been programmed into the data plane (widely used by implementations)
-- Ref: https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.RouteParentStatus
--
-- Staleness: when observedGeneration is present and does not match metadata.generation,
-- the condition is considered stale. Conditions without observedGeneration are not treated as stale.
-- Ref: https://gateway-api.sigs.k8s.io/guides/implementers/#conditions
--
-- Argo CD health mapping (consistent with HTTPRoute/GRPCRoute):
--   Accepted=False or ResolvedRefs=False                 -> Degraded
--   Programmed exists and status != True                 -> Progressing
--   Accepted=True AND ResolvedRefs=True on a fresh parent -> Healthy
--   No status / no parents / required conditions missing  -> Progressing
--
-- NOTE: Only status="False" is treated as a hard failure for Accepted/ResolvedRefs.
-- status="Unknown" is considered non-failing, consistent with existing HTTPRoute/GRPCRoute
-- health scripts. Changing Unknown semantics would require aligning all Gateway API Route
-- scripts and is out of scope for this PR.
--
-- References:
--   TLSRoute spec:              https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.TLSRoute
--   TLSRoute guide:             https://gateway-api.sigs.k8s.io/api-types/tlsroute/
--   RouteParentStatus:          https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.RouteParentStatus
--   observedGeneration pattern: https://gateway-api.sigs.k8s.io/guides/implementers/#conditions

local hs = {}

function findCondition(conditions, conditionType)
  for _, condition in ipairs(conditions) do
    if condition.type == conditionType then
      return condition
    end
  end
  return nil
end

function conditionMessage(condition, conditionType)
  if condition ~= nil and condition.message ~= nil and condition.message ~= "" then
    return condition.message
  end
  return "Failed condition: " .. conditionType
end

function getParentName(parent)
  if parent.parentRef ~= nil and parent.parentRef.name ~= nil then
    return parent.parentRef.name
  end
  return ""
end

-- Skip parent conditions that do not match the current generation (stale status).
-- When observedGeneration is present and mismatches, the parent is skipped.
-- Conditions without observedGeneration are accepted (not treated as stale).
-- Ref: https://gateway-api.sigs.k8s.io/guides/implementers/#conditions
function isParentGenerationObserved(obj, parent)
  if obj.metadata == nil or obj.metadata.generation == nil then
    return true
  end

  if parent.conditions == nil or #parent.conditions == 0 then
    return false
  end

  for _, condition in ipairs(parent.conditions) do
    if condition.observedGeneration ~= nil then
      if condition.observedGeneration ~= obj.metadata.generation then
        return false
      end
    end
  end

  return true
end

if obj.status ~= nil and obj.status.parents ~= nil then
  local hasHealthyParent = false
  local progressingMsg = ""
  local waitingMsg = ""

  for _, parent in ipairs(obj.status.parents) do
    if parent.conditions ~= nil and #parent.conditions > 0 and isParentGenerationObserved(obj, parent) then
      local name = getParentName(parent)
      local resolvedRefs = findCondition(parent.conditions, "ResolvedRefs")
      local accepted = findCondition(parent.conditions, "Accepted")
      local programmed = findCondition(parent.conditions, "Programmed")

      if resolvedRefs ~= nil and resolvedRefs.status == "False" then
        hs.status = "Degraded"
        hs.message = "Parent " .. name .. ": " .. conditionMessage(resolvedRefs, "ResolvedRefs")
        return hs
      end

      if accepted ~= nil and accepted.status == "False" then
        hs.status = "Degraded"
        hs.message = "Parent " .. name .. ": " .. conditionMessage(accepted, "Accepted")
        return hs
      end

      if programmed ~= nil and programmed.status ~= "True" then
        if progressingMsg == "" then
          progressingMsg = "Parent " .. name .. ": " .. (programmed.message or "Route is still being programmed")
        end
      elseif accepted ~= nil and accepted.status == "True" and resolvedRefs ~= nil and resolvedRefs.status == "True" then
        hasHealthyParent = true
      elseif waitingMsg == "" then
        waitingMsg = "Parent " .. name .. ": Waiting for TLSRoute conditions"
      end
    end
  end

  if progressingMsg ~= "" then
    hs.status = "Progressing"
    hs.message = progressingMsg
    return hs
  end

  if hasHealthyParent then
    hs.status = "Healthy"
    hs.message = "TLSRoute is healthy"
    return hs
  end

  if waitingMsg ~= "" then
    hs.status = "Progressing"
    hs.message = waitingMsg
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for TLSRoute status"
return hs
