hs = {
  status = "Progressing",
  message = "Update in progress"
}
if obj.status ~= nil then
  if obj.status.statuses ~= nil then
  for i, namespace in pairs(obj.status.statuses) do

      -- Accepted
      if namespace.state == "Accepted" or namespace.state == "1" then
        hs.status = "Healthy"
        hs.message = "The resource has been validated"
        return hs
      end

      -- Warning
      if namespace.state == "Warning" or namespace.state == "3" then
        hs.status = "Degraded"
        hs.message = namespace.reason
        return hs
      end

      -- Pending
      if namespace.state == "Pending" or namespace.state == "0" then
        hs.status = "Suspended"
        hs.message = "The resource has not yet been validated"
        return hs
      end

      -- Rejected
      if namespace.state == "Rejected" or namespace.state == "2" then
        hs.status = "Degraded"
        hs.message = namespace.reason
        return hs
      end
    end
  end
end
return hs
