//@ts-check
'use strict';

/**
 * Rewrites rootDirs-relative imports to external/bufbuild in proto files.
 * Proto files use:
 *   import { ... } from "../../../external/bufbuild/protobuf.js"
 * This loader rewrites them to:
 *   import { ... } from "@bufbuild/protobuf"
 * so ts-loader emits require("@bufbuild/protobuf") that webpack can resolve.
 * @param {string} source
 * @returns {string}
 */
module.exports = function rewriteBufbuildLoader(source) {
	return source.replace(
		/(from\s+["'])(\.\.\/)*external\/bufbuild\/protobuf\.js(["'])/g,
		'$1@bufbuild/protobuf$3'
	);
};
