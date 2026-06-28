local actions = {}

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
end

if not(completed) and obj.spec.suspend then
  actions["resume"] = {["iconClass"] = "fa fa-fw fa-play" }
elseif not(completed) and (obj.spec.suspend == nil or not(obj.spec.suspend)) then
  actions["suspend"] = {["iconClass"] = "fa fa-fw fa-pause" }
end

if not(completed) and obj.status ~= nil then
  actions["terminate"] = {["iconClass"] = "fa fa-fw fa-stop" }
end

return actions
