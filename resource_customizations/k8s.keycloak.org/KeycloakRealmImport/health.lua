local hs = {
  status = "Progressing",
  message = "Waiting for the realm import to start",
}

if obj.status == nil or obj.status.conditions == nil then
  return hs
end

-- Keycloak 21 and earlier write the condition status as a boolean.
local function isTrue(condition)
  return condition ~= nil and (condition.status == "True" or condition.status == true)
end

-- The operator also sets HasErrors while it waits for the Keycloak StatefulSet to be created and become ready.
-- From 23.0.0 to 26.3.3 the second message is "Deployment not yet ready, waiting for it to be ready".
-- "Keycloak Deployment not found" is the message of Keycloak 21 and earlier.
-- https://github.com/keycloak/keycloak/blob/26.8.1/operator/src/main/java/org/keycloak/operator/controllers/KeycloakRealmImportController.java
local waitingMessages = {
  "No existing Deployment found",
  "Deployment not yet ready",
  "Keycloak Deployment not found",
}

local done
local started
local hasErrors
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Done" then
    done = condition
  elseif condition.type == "Started" then
    started = condition
  elseif condition.type == "HasErrors" then
    hasErrors = condition
  end
end

if isTrue(done) then
  hs.status = "Healthy"
  hs.message = "Realm imported"
  return hs
end

-- "Import Job failed" is set after the first failed pod, while the Job may still retry. The status looks the same
-- once the retries are used up, so it is reported as Degraded.
if isTrue(hasErrors) then
  local message = hasErrors.message or ""
  hs.message = message
  for _, waiting in ipairs(waitingMessages) do
    if string.find(message, waiting, 1, true) ~= nil then
      return hs
    end
  end
  hs.status = "Degraded"
  return hs
end

if isTrue(started) and started.message ~= nil and started.message ~= "" then
  hs.message = started.message
end
return hs
