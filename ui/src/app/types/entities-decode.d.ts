declare module 'entities/decode' {
    const decode: any; // Assuming decode is a function or value
    export default decode;

    // If EntityDecoder is used as a type by parse5
    export type EntityDecoder = any; 

    // If it's also used as a value (e.g., a class constructor), you might need this too:
    // export const EntityDecoder: { new (...args: any[]): any; /* other static props */ };
} 