#ifndef MSGFMT_MSGFMT_H
#define MSGFMT_MSGFMT_H

#include <string>
#include <vector>

namespace msgfmt {

struct Range {
    std::string style;
    size_t offset = 0;
    size_t length = 0;
    std::string url;

    bool operator==(const Range& other) const {
        return style == other.style &&
               offset == other.offset &&
               length == other.length &&
               url == other.url;
    }
};

struct ParsedResult {
    std::string text;
    std::vector<Range> ranges;
};

bool is_valid_utf8(const std::string& input);
std::string escape(const std::string& input);
ParsedResult parse(const std::string& input);
std::string to_json(const ParsedResult& result);

}

#endif
