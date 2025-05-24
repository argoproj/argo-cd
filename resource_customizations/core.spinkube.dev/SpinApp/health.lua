hs = {}

-- Check if status exists
if not obj.status then
  hs.status = "Progressing"
  hs.message = "Waiting for status to be available"
  return hs
end

-- Initialize variables for conditions
local available = false
local progressing = false
local availableMessage = ""
local progressingMessage = ""

-- Check conditions - prioritize failure conditions first
if obj.status.conditions then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Progressing" then
      -- Check for timeout or failure in progressing condition first
      if condition.status == "False" and (condition.reason == "ProgressDeadlineExceeded" or string.find(string.lower(condition.message or ""), "timeout") or string.find(string.lower(condition.message or ""), "failed")) then
        hs.status = "Degraded"
        hs.message = condition.message or "Application deployment has failed"
        return hs
      end
      -- If progressing is true, mark it (any progressing=true condition wins)
      if condition.status == "True" then
        progressing = true
        progressingMessage = condition.message or "Application progress status"
      end
    elseif condition.type == "Available" then
      -- For available, we want all to be true, so any false condition wins
      if condition.status == "True" then
        available = true
        availableMessage = condition.message or "Application availability status"
      else
        available = false
        availableMessage = condition.message or "Application is not available"
      end
    end
  end
end

-- Check ready replicas if specified
local readyReplicas = obj.status.readyReplicas or 0
local desiredReplicas = obj.spec.replicas or 1

-- Determine status based on conditions
if not available then
  hs.status = "Degraded"
  hs.message = availableMessage or "Application is not available"
  return hs
end

if readyReplicas < desiredReplicas then
  hs.status = "Progressing"
  hs.message = string.format("Waiting for replicas to be ready (%d/%d)", readyReplicas, desiredReplicas)
  return hs
end

if progressing then
  hs.status = "Progressing"
  hs.message = progressingMessage or "Application is still progressing"
  return hs
end

-- All checks passed
hs.status = "Healthy"
hs.message = string.format("Application is healthy with %d/%d replicas ready", readyReplicas, desiredReplicas)

return hs