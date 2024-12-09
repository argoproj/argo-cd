export const getBase = () => {
    const bases = document.getElementsByTagName('base');

    return bases.length > 0 ? bases[0].getAttribute('href') || '/' : '/';
};
