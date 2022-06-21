import {ExternalLink, InvalidExternalLinkError} from './application-urls';

test('rejects malicious URLs', () => {
    expect(() => {
        const _ = new ExternalLink('javascript:alert("hi")');
    }).toThrowError(InvalidExternalLinkError);
    expect(() => {
        const _ = new ExternalLink('data:text/html;<h1>hi</h1>');
    }).toThrowError(InvalidExternalLinkError);
});

test('allows absolute URLs', () => {
    expect(new ExternalLink('https://localhost:8080/applications').ref).toEqual('https://localhost:8080/applications');
});

test('allows relative URLs', () => {
    // @ts-ignore
    window.location = new URL('https://localhost:8080/applications');
    expect(new ExternalLink('/applications').ref).toEqual('/applications');
});
