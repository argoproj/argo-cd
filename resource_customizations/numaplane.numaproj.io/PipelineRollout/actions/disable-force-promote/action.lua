if (obj.spec.strategy == nil) then
    obj.spec.strategy = {}
    obj.spec.strategy.progressive = {}
elseif (obj.spec.strategy.progressive == nil) then
    obj.spec.strategy.progressive = {}
end

obj.spec.strategy.progressive.forcePromote = false
return obj 