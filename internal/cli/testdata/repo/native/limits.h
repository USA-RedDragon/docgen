#define LAMP_PORT 4001
#define LAMP_FLAGS (1 << 3)
#define LAMP_RETRIES 3u
#define LAMP_BUFFER 0x400UL
#define LAMP_NAME "lamp" /* shown on the label */

#define LAMP_MASK \
	(LAMP_FLAGS | \
	 0x10)
