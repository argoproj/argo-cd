local hs = {}

local function podMessage()
  if obj.status ~= nil and obj.status.message ~= nil then
    return obj.status.message
  end
  return ""
end

local function isPodReady()
  if obj.status == nil or obj.status.conditions == nil then
    return false
  end
  for _, c in ipairs(obj.status.conditions) do
    if c.type == "Ready" and c.status == "True" then
      return true
    end
  end
  return false
end

local function waitingReasonDegraded(reason)
  if reason == nil then
    return false
  end
  if string.sub(reason, 1, 3) == "Err" then
    return true
  end
  if string.sub(reason, -5) == "Error" then
    return true
  end
  if string.sub(reason, -7) == "BackOff" then
    return true
  end
  return false
end

local function getFailMessage(ctr)
  if ctr.state ~= nil and ctr.state.terminated ~= nil then
    local term = ctr.state.terminated
    if term.message ~= nil and term.message ~= "" then
      return term.message
    end
    if term.reason == "OOMKilled" then
      return term.reason
    end
    if term.exitCode ~= nil and term.exitCode ~= 0 then
      return string.format('container %q failed with exit code %d', ctr.name, term.exitCode)
    end
  end
  return ""
end

local restartPolicy = "Always"
if obj.spec ~= nil and obj.spec.restartPolicy ~= nil then
  restartPolicy = obj.spec.restartPolicy
end

if restartPolicy == "Always" and obj.status ~= nil and obj.status.containerStatuses ~= nil then
  local messages = {}
  local degraded = false
  for _, containerStatus in ipairs(obj.status.containerStatuses) do
    if containerStatus.state ~= nil and containerStatus.state.waiting ~= nil then
      local waiting = containerStatus.state.waiting
      if waitingReasonDegraded(waiting.reason) then
        degraded = true
        table.insert(messages, waiting.message or "")
      end
    end
  end
  if degraded then
    hs.status = "Degraded"
    hs.message = table.concat(messages, ", ")
    return hs
  end
end

local phase = ""
if obj.status ~= nil then
  phase = obj.status.phase or ""
end

if phase == "Pending" then
  hs.status = "Progressing"
  hs.message = podMessage()
  return hs
end
if phase == "Succeeded" then
  hs.status = "Healthy"
  hs.message = podMessage()
  return hs
end
if phase == "Failed" then
  local msg = podMessage()
  if msg ~= "" then
    hs.status = "Degraded"
    hs.message = msg
    return hs
  end
  local containers = {}
  if obj.status ~= nil then
    if obj.status.initContainerStatuses ~= nil then
      for _, ctr in ipairs(obj.status.initContainerStatuses) do
        table.insert(containers, ctr)
      end
    end
    if obj.status.containerStatuses ~= nil then
      for _, ctr in ipairs(obj.status.containerStatuses) do
        table.insert(containers, ctr)
      end
    end
  end
  for _, ctr in ipairs(containers) do
    local failMsg = getFailMessage(ctr)
    if failMsg ~= "" then
      hs.status = "Degraded"
      hs.message = failMsg
      return hs
    end
  end
  hs.status = "Degraded"
  hs.message = ""
  return hs
end
if phase == "Running" then
  if restartPolicy == "Always" then
    if isPodReady() then
      hs.status = "Healthy"
      hs.message = podMessage()
      return hs
    end
    if obj.status ~= nil and obj.status.containerStatuses ~= nil then
      for _, ctrStatus in ipairs(obj.status.containerStatuses) do
        if ctrStatus.lastState ~= nil and ctrStatus.lastState.terminated ~= nil then
          hs.status = "Degraded"
          hs.message = podMessage()
          return hs
        end
      end
    end
    hs.status = "Progressing"
    hs.message = podMessage()
    return hs
  end
  if restartPolicy == "OnFailure" or restartPolicy == "Never" then
    hs.status = "Progressing"
    hs.message = podMessage()
    return hs
  end
end

hs.status = "Unknown"
hs.message = podMessage()
return hs
