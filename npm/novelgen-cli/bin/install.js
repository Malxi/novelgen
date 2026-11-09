'use strict';

const { ensureBinary } = require('./download');

ensureBinary().catch((error) => {
  console.error(`novelgen: ${error.message}`);
  process.exit(1);
});
