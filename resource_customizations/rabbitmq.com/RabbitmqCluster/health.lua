local function getCondition(conditions, condType)
  if not conditions then return nil end
  for i, c in ipairs(conditions) do
    if c.type == condType then
      return c
    end
  end
  return nil
end

local function health(resource)
  -- Default
  local status = "Progressing"
  local message = ""

  if not resource or not resource.status then
    return { status = status, message = "No status available" }
  end

  local conditions = resource.status.conditions or {}

  local clusterAvailable = getCondition(conditions, "ClusterAvailable")
  local allReplicasReady = getCondition(conditions, "AllReplicasReady")

  -- If any required condition is explicitly False => Degraded
  if (clusterAvailable and clusterAvailable.status == "False")
      or (allReplicasReady and allReplicasReady.status == "False") then
    local msgParts = {}
    if clusterAvailable and clusterAvailable.message then table.insert(msgParts, clusterAvailable.message) end
    if allReplicasReady and allReplicasReady.message then table.insert(msgParts, allReplicasReady.message) end
    local msg = (#msgParts > 0) and table.concat(msgParts, " | ") or "One or more conditions reported False"
    return { status = "Degraded", message = msg }
  end

  -- Treat Unknown for these conditions as transient/Progressing.
  -- Operators commonly set Unknown briefly while reconciling; mapping Unknown -> Degraded
  -- causes short-lived false negatives (e.g. argocd app wait --health failing).
  if (clusterAvailable and clusterAvailable.status == "Unknown")
      or (allReplicasReady and allReplicasReady.status == "Unknown") then
    local msg = "Cluster is initializing (condition(s) Unknown)"
    return { status = "Progressing", message = msg }
  end

  -- If both conditions are True => Healthy
  if (clusterAvailable and clusterAvailable.status == "True")
      and (allReplicasReady and allReplicasReady.status == "True") then
    return { status = "Healthy", message = "Cluster available and all replicas ready" }
  end

  -- Fallbacks: if status has ready/replicas fields you can use them (best-effort)
  -- Keep Progressing as default to avoid false Degraded states.
  return { status = status, message = "Waiting for cluster to become ready" }
end

return { health = health }