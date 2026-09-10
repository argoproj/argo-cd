-- Surface deletion progress while the resource is terminating. You can customize this
-- block, e.g. map known finalizers in obj.metadata.finalizers to clearer messages.
if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil then
  local deletionHs = {}
  deletionHs.status = "Progressing"
  deletionHs.message = "Pending deletion"
  if obj.metadata.finalizers ~= nil and #obj.metadata.finalizers > 0 then
    deletionHs.message = "Pending deletion; blocked by finalizers: " .. table.concat(obj.metadata.finalizers, ", ")
  end
  return deletionHs
end

local health_status = {}

health_status.status = "Progressing"
health_status.message = "Waiting for LLMInferenceService to report status..."

if obj.status == nil or obj.status.conditions == nil then
  return health_status
end

-- KServe reconciles the LLMInferenceService as a Knative "living" resource: the
-- dependent conditions (PresetsCombined, WorkloadsReady, RouterReady and their
-- sub-conditions) are aggregated into the top-level "Ready" condition. We assess
-- health from "Ready" and surface the not-ready dependents in the message.
local ready = nil
local stopped = false
local msg = ""

for _, condition in ipairs(obj.status.conditions) do
  -- A stopped LLMInferenceService is signalled by the controller setting the
  -- reason of its workload/preset conditions to "Stopped".
  if condition.reason == "Stopped" then
    stopped = true
  end

  if condition.type == "Ready" then
    ready = condition
  elseif condition.status ~= "True" then
    if msg ~= "" then
      msg = msg .. "; "
    end
    msg = msg .. condition.type .. ": " .. condition.status
    if condition.reason ~= nil and condition.reason ~= "" then
      msg = msg .. " | " .. condition.reason
    end
    if condition.message ~= nil and condition.message ~= "" then
      msg = msg .. " | " .. condition.message
    end
  end
end

-- Treat the assessment as still in progress when the controller has not yet
-- observed the current spec (generation mismatch). Otherwise a stale Ready=True
-- condition left over from a previous generation would be reported as Healthy
-- before KServe has reconciled the update.
local generation = nil
if obj.metadata ~= nil then
  generation = obj.metadata.generation
end
if generation ~= nil then
  local stale = false
  -- status-level observedGeneration (duckv1.Status.ObservedGeneration)
  if obj.status.observedGeneration ~= nil and obj.status.observedGeneration ~= generation then
    stale = true
  end
  -- condition-level observedGeneration (per-condition, when the controller sets it)
  if ready ~= nil and ready.observedGeneration ~= nil and ready.observedGeneration ~= generation then
    stale = true
  end
  if stale then
    health_status.status = "Progressing"
    health_status.message = "Waiting for LLMInferenceService reconciliation (generation mismatch)"
    return health_status
  end
end

if stopped then
  health_status.status = "Suspended"
  health_status.message = "LLMInferenceService is Stopped"
  return health_status
end

if ready ~= nil and ready.status == "True" then
  health_status.status = "Healthy"
  health_status.message = "LLMInferenceService is healthy."
  return health_status
end

if ready ~= nil and ready.status == "False" then
  health_status.status = "Degraded"
else
  -- Ready is Unknown or has not been reported yet.
  health_status.status = "Progressing"
end

if msg ~= "" then
  health_status.message = msg
elseif ready ~= nil and ready.message ~= nil and ready.message ~= "" then
  health_status.message = ready.message
end

return health_status
