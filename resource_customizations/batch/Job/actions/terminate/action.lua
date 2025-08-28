local os = require("os")

local completed = false
if obj.status ~= nil then

  if obj.status.conditions ~= nil then
    for i, condition in pairs(obj.status.conditions) do
      if condition.type == "Complete" and condition.status == "True" then
        completed = true
      elseif condition.type == "Failed" and condition.status == "True" then
        completed = true
      elseif condition.type == "FailureTarget" and condition.status == "True" then
        completed = true
      elseif condition.type == "SuccessCriteriaMet" and condition.status == "True" then
        completed = true
      end
    end
  end

  if not(completed) then
    obj.status.conditions = obj.status.conditions or {}
    table.insert(obj.status.conditions, {
      lastTransitionTime = os.date("!%Y-%m-%dT%XZ"),
      message = "Job was terminated explicitly through Argo CD",
      reason = "ManuallyTerminated",
      status = "True",
      type = "FailureTarget"
    })
  end

end
return obj
