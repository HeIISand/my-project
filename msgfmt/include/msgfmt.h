#pragma once

#include <string>
#include <vector>

namespace msgfmt {

struct Range {
    std::string style;
    size_t offset = 0;
    size_t length = 0;
    std::string url;
};

struct ParsedResult {
    std::string text;
    std::vector<Range> ranges;
};

bool is_valid_utf8(const std::string& input);
std::string escape(const std::string& input);

}
