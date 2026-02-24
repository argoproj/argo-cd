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
--   Accepted/ResolvedRefs = False              -> Degraded
--   Programmed exists and status != True       -> Progressing
--   Otherwise (current generation, no failure) -> Healthy
--   No status / no parents                     -> Progressing
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

-- Only treat status="False" as a hard failure for consistency with HTTPRoute/GRPCRoute.
-- status="Unknown" is considered non-failing and may still be gated by Programmed/generation checks.
function checkConditions(conditions, conditionType)
  for _, condition in ipairs(conditions) do
    if condition.type == conditionType and condition.status == "False" then
      return false, condition.message or ("Failed condition: " .. conditionType)
    end
  end
  return true
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

if obj.status ~= nil then
  if obj.status.parents ~= nil then
    for _, parent in ipairs(obj.status.parents) do
      if parent.conditions ~= nil then
        if not isParentGenerationObserved(obj, parent) then
          goto continue
        end

        -- parentRef and parentRef.name may be omitted depending on implementation;
        -- fall back to empty name / generic messages to keep health output deterministic.
        local parentName = ""
        if parent.parentRef ~= nil and parent.parentRef.name ~= nil then
          parentName = parent.parentRef.name
        end

        local resolvedRefsFalse, resolvedRefsMsg = checkConditions(parent.conditions, "ResolvedRefs")
        local acceptedFalse, acceptedMsg = checkConditions(parent.conditions, "Accepted")

        if not resolvedRefsFalse then
          hs.status = "Degraded"
          hs.message = "Parent " .. parentName .. ": " .. resolvedRefsMsg
          return hs
        end

        if not acceptedFalse then
          hs.status = "Degraded"
          hs.message = "Parent " .. parentName .. ": " .. acceptedMsg
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
          hs.message = "Parent " .. parentName .. ": " .. progressingMsg
          return hs
        end

        ::continue::
      end
    end

    -- If we found at least one parent with conditions for the current generation and no hard failures,
    -- consider the Route healthy (consistent with existing HTTPRoute/GRPCRoute health scripts).
    if #obj.status.parents > 0 then
      for _, parent in ipairs(obj.status.parents) do
        if parent.conditions ~= nil and #parent.conditions > 0 then
          if isParentGenerationObserved(obj, parent) then
            hs.status = "Healthy"
            hs.message = "TLSRoute is healthy"
            return hs
          end
        end
      end
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for TLSRoute status"
return hs
