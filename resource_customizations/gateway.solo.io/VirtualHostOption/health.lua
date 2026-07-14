hs = {
  status = "Progressing",
  message = "Update in progress"
}

function getStatus(status)
  -- Accepted
  if status.state == "Accepted" or status.state == 1 then
    hs.status = "Healthy"
    hs.message = "The resource has been validated"
    return hs
  end

  -- Warning
  if status.state == "Warning" or status.state == 3 then
    hs.status = "Degraded"
    hs.message = status.reason
    return hs
  end

  -- Pending
  if status.state == "Pending" or status.state == 0 then
    hs.status = "Suspended"
    hs.message = "The resource has not yet been validated"
    return hs
  end

  -- Rejected
  if status.state == "Rejected" or status.state == 2 then
    hs.status = "Degraded"
    hs.message = status.reason
    return hs
  end

  return hs
end

if obj.status ~= nil then
  -- Namespaced version of status
  if obj.status.statuses ~= nil then
    for i, namespace in pairs(obj.status.statuses) do
      hs = getStatus(namespace)
      if hs.status ~= "Progressing" then
        return hs
      end
    end
  end

  -- Older non-namespaced version of status
  if obj.status.state ~= nil then
    hs = getStatus(obj.status)
  end
end
return hs
