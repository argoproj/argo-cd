// TODO: This needs to be polyfilled until jest-environment-jsdom decides to pull in a version of jsdom that's >=27.4.0
const {TextEncoder, TextDecoder} = require('util');

if (typeof globalThis.TextEncoder === 'undefined') {
    globalThis.TextEncoder = TextEncoder;
}
if (typeof globalThis.TextDecoder === 'undefined') {
    globalThis.TextDecoder = TextDecoder;
}
