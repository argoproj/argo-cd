import {ExternalLink, ExternalLinks, InvalidExternalLinkError} from './application-urls';

test('rejects malicious URLs', () => {
    expect(() => {
        const _ = new ExternalLink('javascript:alert("hi")');
    }).toThrowError(InvalidExternalLinkError);
    expect(() => {
        const _ = new ExternalLink('data:text/html;<h1>hi</h1>');
    }).toThrowError(InvalidExternalLinkError);
    expect(() => {
        const _ = new ExternalLink('title|data:text/html;<h1>hi</h1>');
    }).toThrowError(InvalidExternalLinkError);
    expect(() => {
        const _ = new ExternalLink('data:title|data:text/html;<h1>hi</h1>');
    }).toThrowError(InvalidExternalLinkError);

    expect(() => {
        const _ = new ExternalLink('data:title|https://localhost:8080/applications');
    }).not.toThrowError(InvalidExternalLinkError);
});

test('allows absolute URLs', () => {
    expect(new ExternalLink('https://localhost:8080/applications').ref).toEqual('https://localhost:8080/applications');
});

test('allows relative URLs', () => {
    // @ts-ignore
    window.location = new URL('https://localhost:8080/applications');
    expect(new ExternalLink('/applications').ref).toEqual('/applications');
});

test('URLs format', () => {
    expect(new ExternalLink('https://localhost:8080/applications')).toEqual({
        ref: 'https://localhost:8080/applications',
        title: 'https://localhost:8080/applications',
    });
    expect(new ExternalLink('title|https://localhost:8080/applications')).toEqual({
        ref: 'https://localhost:8080/applications',
        title: 'title',
    });
});

test('malicious URLs from list to be removed', () => {
    const urls: string[] = ['javascript:alert("hi")', 'https://localhost:8080/applications'];
    const links = ExternalLinks(urls);

    expect(links).toHaveLength(1);
    expect(links).toContainEqual({
        ref: 'https://localhost:8080/applications',
        title: 'https://localhost:8080/applications',
    });
});

test('list to be sorted', () => {
    const urls: string[] = ['https://a', 'https://b', 'a|https://c', 'z|https://c', 'x|https://d', 'x|https://c'];
    const links = ExternalLinks(urls);

    // 'a|https://c',
    // 'x|https://c',
    // 'x|https://d',
    // 'z|https://c',
    // 'https://a',
    // 'https://b',
    expect(links).toHaveLength(6);
    expect(links[0].title).toEqual('a');
    expect(links[1].title).toEqual('x');
    expect(links[1].ref).toEqual('https://c');
    expect(links[2].title).toEqual('x');
    expect(links[2].ref).toEqual('https://d');
    expect(links[3].title).toEqual('z');
    expect(links[4].title).toEqual('https://a');
    expect(links[5].title).toEqual('https://b');
});
