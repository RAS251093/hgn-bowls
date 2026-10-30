interface ImportMetaEnv {
	readonly PUBLIC_API_URL?: string;
	readonly PUBLIC_AUTH0_DOMAIN?: string;
	readonly PUBLIC_AUTH0_CLIENT_ID?: string;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
}
