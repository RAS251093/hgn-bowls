interface ImportMetaEnv {
	readonly PUBLIC_API_URL?: string;
	readonly PUBLIC_AUTH0_DOMAIN?: string;
	readonly PUBLIC_AUTH0_CLIENT_ID?: string;
	readonly PUBLIC_DEPLOY_POLL_MS?: string;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
}
