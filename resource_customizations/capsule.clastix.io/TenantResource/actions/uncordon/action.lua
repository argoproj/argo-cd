if obj.spec ~= nil and obj.spec.cordoned ~= nil and obj.spec.cordoned then
  obj.spec.cordoned = false
end
return obj
